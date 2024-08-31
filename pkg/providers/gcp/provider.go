package gcp

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/general"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

const (
	UpdateQueueSize         = 100
	ResourcePollingInterval = 2 * time.Second
	DebugFilePath           = "/tmp/andaime-debug.log"
	DebugFilePermissions    = 0644
	WaitingForMachinesTime  = 1 * time.Minute
)

type UpdateAction struct {
	MachineName string
	UpdateData  UpdatePayload
	UpdateFunc  func(*models.Machine,
		UpdatePayload,
	)
}

func NewUpdateAction(
	machineName string,
	updateData UpdatePayload,
) UpdateAction {
	l := logger.Get()
	updateFunc := func(machine *models.Machine, update UpdatePayload) {
		if update.UpdateType == UpdateTypeResource {
			machine.SetResourceState(update.ResourceType.ResourceString, update.ResourceState)
		} else if update.UpdateType == UpdateTypeService {
			machine.SetServiceState(update.ServiceType.Name, update.ServiceState)
		} else {
			l.Errorf("Invalid update type: %s", update.UpdateType)
		}
	}
	return UpdateAction{
		MachineName: machineName,
		UpdateData:  updateData,
		UpdateFunc:  updateFunc,
	}
}

type UpdatePayload struct {
	UpdateType    UpdateType
	ServiceType   models.ServiceType
	ServiceState  models.ServiceState
	ResourceType  models.AzureResourceTypes
	ResourceState models.AzureResourceState
	Complete      bool
}

func (u *UpdatePayload) String() string {
	if u.UpdateType == UpdateTypeResource {
		return fmt.Sprintf("%s: %s", u.UpdateType, u.ResourceType.ResourceString)
	}
	return fmt.Sprintf("%s: %s", u.UpdateType, u.ServiceType.Name)
}

type UpdateType string

const (
	UpdateTypeResource UpdateType = "resource"
	UpdateTypeService  UpdateType = "service"
	UpdateTypeComplete UpdateType = "complete"
)

// AzureProvider wraps the Azure deployment functionality
type GCPProviderer interface {
	general.Providerer
	GetGCPClient() GCPClienter
	SetGCPClient(client GCPClienter)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)

	EnsureProject(
		ctx context.Context,
		projectID string,
	) (string, error)
	DestroyProject(
		ctx context.Context,
		projectID string,
	) error
	ListProjects(
		ctx context.Context,
	) ([]*resourcemanagerpb.Project, error)
	ListAllAssetsInProject(
		ctx context.Context,
		projectID string,
	) ([]*assetpb.Asset, error)
	DeployResources(ctx context.Context) error
	ProvisionPackagesOnMachines(ctx context.Context) error
	ProvisionBacalhau(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	StartResourcePolling(ctx context.Context)
	CheckAuthentication(ctx context.Context) error
	EnableAPI(ctx context.Context, projectID, apiName string) error
	CreateVPCNetwork(ctx context.Context, projectID, networkName string) error
	CreateSubnet(ctx context.Context, projectID, networkName, subnetName, cidr string) error
	CreateFirewallRules(ctx context.Context, projectID, networkName string) error
	CreateStorageBucket(ctx context.Context, projectID, bucketName string) error
	CreateVM(ctx context.Context, projectID string, vmConfig map[string]string) (string, error)
}

type GCPProvider struct {
	Client              GCPClienter
	Config              *viper.Viper
	SSHClient           sshutils.SSHClienter
	SSHUser             string
	SSHPort             int
	updateQueue         chan UpdateAction
	serviceMutex        sync.Mutex //nolint:unused
	servicesProvisioned bool       //nolint:unused
}

var NewGCPProviderFunc = NewGCPProvider

// NewAzureProvider creates a new AzureProvider instance
func NewGCPProvider() (GCPProviderer, error) {
	config := viper.GetViper()
	if !config.IsSet("gcp") {
		return nil, fmt.Errorf("gcp configuration is required")
	}

	if !config.IsSet("gcp.organization_id") {
		return nil, fmt.Errorf(
			"gcp.organization_id is required. Please specify a parent organization or folder - " +
				"use 'gcloud organizations list' to get a list of your organization ids",
		)
	}
	organizationID := config.GetString("gcp.organization_id")

	// Check for SSH keys
	sshPublicKeyPath := config.GetString("general.ssh_public_key_path")
	sshPrivateKeyPath := config.GetString("general.ssh_private_key_path")
	if sshPublicKeyPath == "" {
		return nil, fmt.Errorf("general.ssh_public_key_path is required")
	}
	if sshPrivateKeyPath == "" {
		return nil, fmt.Errorf("general.ssh_private_key_path is required")
	}

	// Expand the paths
	expandedPublicKeyPath, err := homedir.Expand(sshPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand public key path: %w", err)
	}
	expandedPrivateKeyPath, err := homedir.Expand(sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand private key path: %w", err)
	}

	// Check if the files exist
	if _, err := os.Stat(expandedPublicKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SSH public key file does not exist: %s", expandedPublicKeyPath)
	}
	if _, err := os.Stat(expandedPrivateKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SSH private key file does not exist: %s", expandedPrivateKeyPath)
	}

	// Update the config with the expanded paths
	config.Set("general.ssh_public_key_path", expandedPublicKeyPath)
	config.Set("general.ssh_private_key_path", expandedPrivateKeyPath)

	client, err := GetGCPClient(context.Background(), organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP client: %w", err)
	}

	sshUser := config.GetString("general.ssh_user")
	if sshUser == "" {
		sshUser = "gcpuser" // Default SSH user for GCP VMs
	}

	sshPort := config.GetInt("general.ssh_port")
	if sshPort == 0 {
		sshPort = 22 // Default SSH port
	}

	provider := &GCPProvider{
		Client:      client,
		Config:      config,
		SSHUser:     sshUser,
		SSHPort:     sshPort,
		updateQueue: make(chan UpdateAction, UpdateQueueSize),
	}

	return provider, nil
}

func (p *GCPProvider) GetConfig() *viper.Viper {
	return p.Config
}

func (p *GCPProvider) SetConfig(config *viper.Viper) {
	p.Config = config
}

func (p *GCPProvider) GetGCPClient() GCPClienter {
	return p.Client
}

func (p *GCPProvider) SetGCPClient(client GCPClienter) {
	p.Client = client
}

func (p *GCPProvider) GetSSHClient() sshutils.SSHClienter {
	return p.SSHClient
}

func (p *GCPProvider) SetSSHClient(client sshutils.SSHClienter) {
	p.SSHClient = client
}

func (p *GCPProvider) EnsureProject(
	ctx context.Context,
	projectID string,
) (string, error) {
	return p.Client.EnsureProject(ctx, projectID)
}

func (p *GCPProvider) DestroyProject(
	ctx context.Context,
	projectID string,
) error {
	return p.Client.DestroyProject(ctx, projectID)
}

func (p *GCPProvider) ListProjects(
	ctx context.Context,
) ([]*resourcemanagerpb.Project, error) {
	return p.Client.ListProjects(ctx, &resourcemanagerpb.ListProjectsRequest{})
}

func (p *GCPProvider) StartResourcePolling(ctx context.Context) {
	l := logger.Get()
	if err := p.Client.StartResourcePolling(ctx); err != nil {
		l.Errorf("Failed to start resource polling: %v", err)
	}
}

func (p *GCPProvider) DeployResources(ctx context.Context) error {
	return p.Client.DeployResources(ctx)
}

func (p *GCPProvider) ProvisionPackagesOnMachines(ctx context.Context) error {
	return p.Client.ProvisionPackagesOnMachines(ctx)
}

func (p *GCPProvider) ProvisionBacalhau(ctx context.Context) error {
	return p.Client.ProvisionBacalhau(ctx)
}

func (p *GCPProvider) FinalizeDeployment(ctx context.Context) error {
	return p.Client.FinalizeDeployment(ctx)
}

func (p *GCPProvider) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	return p.Client.ListAllAssetsInProject(ctx, projectID)
}

func (p *GCPProvider) CheckAuthentication(ctx context.Context) error {
	return p.Client.CheckAuthentication(ctx)
}

func (p *GCPProvider) EnableAPI(ctx context.Context, projectID, apiName string) error {
	return p.Client.EnableAPI(ctx, projectID, apiName)
}

func (p *GCPProvider) CreateVPCNetwork(ctx context.Context, projectID, networkName string) error {
	return p.Client.CreateVPCNetwork(ctx, projectID, networkName)
}

func (p *GCPProvider) CreateSubnet(
	ctx context.Context,
	projectID, networkName, subnetName, cidr string,
) error {
	return p.Client.CreateSubnet(ctx, projectID, networkName, subnetName, cidr)
}

func (p *GCPProvider) CreateFirewallRules(
	ctx context.Context,
	projectID, networkName string,
) error {
	return p.Client.CreateFirewallRules(ctx, projectID, networkName)
}

func (p *GCPProvider) CreateStorageBucket(ctx context.Context, projectID, bucketName string) error {
	return p.Client.CreateStorageBucket(ctx, projectID, bucketName)
}

func (p *GCPProvider) CreateVM(
	ctx context.Context,
	projectID string,
	vmConfig map[string]string,
) (string, error) {
	return p.Client.CreateVM(ctx, projectID, vmConfig)
}

var (
	gcpClientInstance GCPClienter
	gcpClientOnce     sync.Once
)

func GetGCPClient(ctx context.Context, organizationID string) (GCPClienter, error) {
	var err error
	gcpClientOnce.Do(func() {
		gcpClientInstance, err = NewGCPClient(ctx, organizationID)
	})
	return gcpClientInstance, err
}
