package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/cenkalti/backoff/v4"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"google.golang.org/api/iam/v1"
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
	ResourceType  models.ResourceTypes
	ResourceState models.ResourceState
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

type GCPProvider struct {
	Client              GCPClienter
	CleanupClient       func()
	Config              *viper.Viper
	SSHClient           sshutils.SSHClienter
	SSHUser             string
	SSHPort             int
	updateQueue         chan UpdateAction
	serviceMutex        sync.Mutex //nolint:unused
	servicesProvisioned bool       //nolint:unused
}

var NewGCPProviderFunc = NewGCPProvider

func NewGCPProvider(ctx context.Context) (GCPProviderer, error) {
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

	// Check if a project ID is provided
	projectID := config.GetString("gcp.project_id")
	if projectID == "" {
		// Generate a new project ID if not provided
		projectID = fmt.Sprintf("andaime-project-%s", time.Now().Format("20060102150405"))
		config.Set("gcp.project_id", projectID)
	}

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

	client, cleanup, err := NewGCPClientFunc(ctx, organizationID)
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
		Client:        client,
		CleanupClient: cleanup,
		Config:        config,
		SSHUser:       sshUser,
		SSHPort:       sshPort,
		updateQueue:   make(chan UpdateAction, UpdateQueueSize),
	}

	// Load deployment data from config
	err = provider.loadDeploymentFromConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load deployment from config: %w", err)
	}

	return provider, nil
}

func (p *GCPProvider) loadDeploymentFromConfig() error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// Load project ID
	m.Deployment.ProjectID = p.Config.GetString("gcp.project_id")

	// Load billing account ID
	m.Deployment.GCP.BillingAccountID = p.Config.GetString("gcp.billing_account_id")

	// Load service account email
	m.Deployment.GCP.ServiceAccountEmail = p.Config.GetString("gcp.service_account_email")

	// Load allowed ports
	m.Deployment.AllowedPorts = p.Config.GetIntSlice("gcp.allowed_ports")

	// Initialize the ProjectServiceAccounts map if it's nil
	if m.Deployment.GCP.ProjectServiceAccounts == nil {
		m.Deployment.GCP.ProjectServiceAccounts = make(map[string]models.ServiceAccountInfo)
	}

	// Load project-specific data
	projectsMap := p.Config.GetStringMap("gcp.projects")
	for projectID, projectData := range projectsMap {
		if projectDataMap, ok := projectData.(map[string]interface{}); ok {
			if saData, ok := projectDataMap["service_account"].(map[string]interface{}); ok {
				email, _ := saData["email"].(string)
				key, _ := saData["key"].(string)
				if email != "" && key != "" {
					m.Deployment.GCP.ProjectServiceAccounts[projectID] = models.ServiceAccountInfo{
						Email: email,
						Key:   key,
					}
				}
			}
		}
	}

	return nil
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
	// Create the project
	createdProjectID, err := p.Client.EnsureProject(ctx, projectID)
	if err != nil {
		return "", err
	}

	// Set the project ID in the deployment model
	m := display.GetGlobalModelFunc()
	if m != nil && m.Deployment != nil {
		m.Deployment.ProjectID = createdProjectID
	}

	// Create a service account for the project
	err = p.createServiceAccount(ctx, createdProjectID)
	if err != nil {
		return "", err
	}

	// Create firewall rules for the project
	err = p.CreateFirewallRules(ctx, "default")
	if err != nil {
		return "", fmt.Errorf("failed to create firewall rules: %v", err)
	}

	return createdProjectID, nil
}

func (p *GCPProvider) createServiceAccount(ctx context.Context, projectID string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	l.Infof("Creating service account for project %s", projectID)

	// Create a new service account
	sa, err := p.Client.CreateServiceAccount(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to create service account: %v", err)
	}

	l.Infof("Service account created or found: %s", sa.Email)

	// Ensure the service account exists before creating a key
	retryBackoff := backoff.NewExponentialBackOff()
	retryBackoff.MaxElapsedTime = 1 * time.Minute

	var key *iam.ServiceAccountKey
	err = backoff.Retry(func() error {
		var innerErr error
		key, innerErr = p.Client.CreateServiceAccountKey(ctx, projectID, sa.Email)
		if innerErr != nil {
			l.Warnf("Failed to create service account key, retrying: %v", innerErr)
			return innerErr // This will trigger a retry
		}
		return nil // Success, stop retrying
	}, retryBackoff)

	if err != nil {
		return fmt.Errorf("failed to create service account key after retries: %v", err)
	}

	// Decode the private key
	decodedPrivateKey, err := base64.StdEncoding.DecodeString(key.PrivateKeyData)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %v", err)
	}

	// Store the service account details in the config
	p.Config.Set(fmt.Sprintf("gcp.projects.%s.service_account.email", projectID), sa.Email)
	p.Config.Set(
		fmt.Sprintf("gcp.projects.%s.service_account.key", projectID),
		string(decodedPrivateKey),
	)

	// Update the deployment model
	m.Deployment.GCP.ServiceAccountEmail = sa.Email

	// Save the updated config
	err = p.Config.WriteConfig()
	if err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	l.Infof("Service account credentials stored in config for project %s", projectID)

	return nil
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

func (p *GCPProvider) EnableRequiredAPIs(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID
	if projectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	requiredAPIs := []string{
		"compute.googleapis.com",
		"networkmanagement.googleapis.com",
		"storage-api.googleapis.com",
		"file.googleapis.com",
		"storage.googleapis.com",
	}

	for _, api := range requiredAPIs {
		err := p.EnableAPI(ctx, api)
		if err != nil {
			return fmt.Errorf("failed to enable API %s: %v", api, err)
		}
		l.Infof("Enabled API: %s for project %s", api, projectID)
	}

	return nil
}

func (p *GCPProvider) EnableAPI(ctx context.Context, apiName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID
	if projectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	l.Infof("Checking API status: %s for project: %s", apiName, projectID)

	// First, check if the API is already enabled
	enabled, err := p.Client.IsAPIEnabled(ctx, projectID, apiName)
	if err != nil {
		l.Warnf("Failed to check API status: %v", err)
	} else if enabled {
		l.Infof("API %s is already enabled", apiName)
		return nil
	}

	l.Infof("Attempting to enable API: %s for project: %s", apiName, projectID)

	err = p.Client.EnableAPI(ctx, projectID, apiName)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			l.Warnf(
				"Failed to enable API %s due to permission issues. You may need to enable it manually: %v",
				apiName,
				err,
			)
			return nil
		}
		l.Errorf("Failed to enable API %s: %v", apiName, err)
		return fmt.Errorf("failed to enable API %s: %v", apiName, err)
	}

	l.Infof("Successfully enabled API: %s", apiName)
	return nil
}


func (p *GCPProvider) CreateVPCNetwork(
	ctx context.Context,
	networkName string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	l.Infof("Creating VPC network %s in project %s", networkName, m.Deployment.ProjectID)

	// First, ensure that the Compute Engine API is enabled
	err := p.EnableAPI(ctx, "compute.googleapis.com")
	if err != nil {
		return fmt.Errorf("failed to enable Compute Engine API: %v", err)
	}

	// Define the exponential backoff strategy
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 3 * time.Minute

	// Define the operation to retry
	operation := func() error {
		l.Infof("Attempting to create VPC network %s...", networkName)
		err := p.Client.CreateVPCNetwork(ctx, networkName)
		if err != nil {
			if strings.Contains(err.Error(), "Compute Engine API has not been used") {
				l.Infof("Compute Engine API is not yet active. Retrying...")
				return err // This error will trigger a retry
			}
			return backoff.Permanent(err) // This error will not trigger a retry
		}
		return nil
	}

	// Define the notify function to keep the user updated
	notify := func(err error, duration time.Duration) {
		l.Infof("Attempt to create VPC network failed. Retrying in %v: %v", duration, err)
	}

	// Execute the operation with backoff
	err = backoff.RetryNotify(operation, b, notify)

	if err != nil {
		l.Errorf("Failed to create VPC network after multiple attempts: %v", err)
		return fmt.Errorf("failed to create VPC network: %v", err)
	}

	l.Infof("VPC network %s created successfully", networkName)
	return nil
}

func (p *GCPProvider) CreateFirewallRules(
	ctx context.Context,
	networkName string,
) error {
	l := logger.Get()
	l.Infof("Creating firewall rules for network: %s", networkName)

	err := p.Client.CreateFirewallRules(ctx, networkName)
	if err != nil {
		return fmt.Errorf("failed to create firewall rules: %v", err)
	}

	l.Infof("Firewall rules created successfully for network: %s", networkName)
	return nil
}

func (p *GCPProvider) CreateStorageBucket(
	ctx context.Context,
	bucketName string,
) error {
	return p.Client.CreateStorageBucket(ctx, bucketName)
}

func (p *GCPProvider) TestSSHLiveness(
	ctx context.Context,
	machineName string,
) error {
	l := logger.Get()

	l.Infof("Testing SSH connectivity to %s", machineName)
	m := display.GetGlobalModelFunc()
	mach, ok := m.Deployment.Machines[machineName]
	if !ok {
		return fmt.Errorf("machine %s not found", machineName)
	}

	sshConfig, err := sshutils.NewSSHConfigFunc(
		mach.PublicIP,
		p.SSHPort,
		p.SSHUser,
		[]byte(mach.SSHPrivateKeyMaterial),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %v", err)
	}

	return sshConfig.WaitForSSH(ctx, p.SSHPort, 5*time.Minute)
}

func (p *GCPProvider) getSSHPrivateKeyMaterial() (string, error) {
	privateKeyPath := p.Config.GetString("general.ssh_private_key_path")
	if privateKeyPath == "" {
		return "", fmt.Errorf("SSH private key path is not set")
	}

	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read private key file: %v", err)
	}

	return string(privateKeyBytes), nil
}

func (p *GCPProvider) ListBillingAccounts(ctx context.Context) ([]string, error) {
	return p.Client.ListBillingAccounts(ctx)
}

func (p *GCPProvider) SetBillingAccount(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	l.Infof(
		"Setting billing account to %s for project %s",
		m.Deployment.GCP.BillingAccountID,
		m.Deployment.ProjectID,
	)

	return p.Client.SetBillingAccount(
		ctx,
		m.Deployment.ProjectID,
		m.Deployment.GCP.BillingAccountID,
	)
}

var (
	gcpClientInstance GCPClienter
	gcpClientOnce     sync.Once
)

func GetGCPClient(ctx context.Context, organizationID string) (GCPClienter, func(), error) {
	var err error
	var cleanup func()
	gcpClientOnce.Do(func() {
		gcpClientInstance, cleanup, err = NewGCPClientFunc(ctx, organizationID)
	})
	return gcpClientInstance, cleanup, err
}

func (p *GCPProvider) GetVMExternalIP(
	ctx context.Context,
	projectID, zone, vmName string,
) (string, error) {
	return p.Client.GetVMExternalIP(ctx, projectID, zone, vmName)
}

type GCPVMConfig struct {
	ProjectID         string
	Region            string
	Zone              string
	VMName            string
	MachineType       string
	SSHUser           string
	PublicKeyMaterial string
}
