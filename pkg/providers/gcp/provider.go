// pkg/providers/gcp/provider.go
package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/asset/apiv1/assetpb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/providers/factory"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

// Ensure GCPProvider implements the Providerer interface.
var _ gcp_interface.GCPProviderer = &GCPProvider{}

// NewGCPProviderFactory is the factory function for GCPProvider.
func NewGCPProviderFactory(
	ctx context.Context,
) (common_interface.Providerer, error) {
	projectID := viper.GetString("gcp.project_id")
	if projectID == "" {
		return nil, fmt.Errorf("gcp.project_id is not set in configuration")
	}
	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is not set in configuration")
	}
	billingAccountID := viper.GetString("gcp.billing_account_id")
	if billingAccountID == "" {
		return nil, fmt.Errorf("gcp.billing_account_id is not set in configuration")
	}
	return NewGCPProvider(ctx, projectID, organizationID, billingAccountID)
}

func init() {
	factory.RegisterProvider(models.DeploymentTypeGCP, NewGCPProviderFactory)
}

// Constants related to GCP APIs and configurations
var GCPRequiredAPIs = []string{
	"compute.googleapis.com",
	"networkmanagement.googleapis.com",
	"storage-api.googleapis.com",
	"file.googleapis.com",
	"storage.googleapis.com",
	"cloudasset.googleapis.com",
}

func GetRequiredAPIs() []string {
	return GCPRequiredAPIs
}

// GCPProvider implements the Providerer interface for GCP
type GCPProvider struct {
	ProjectID           string
	OrganizationID      string
	BillingAccountID    string
	Client              gcp_interface.GCPClienter
	ClusterDeployer     common_interface.ClusterDeployerer
	CleanupClient       func()
	Config              *viper.Viper
	SSHClient           sshutils.SSHClienter
	SSHUser             string
	SSHPort             int
	updateQueue         chan display.UpdateAction
	updateProcessorDone chan struct{} //nolint:unused
	updateMutex         sync.Mutex    //nolint:unused
	serviceMutex        sync.Mutex    //nolint:unused
	servicesProvisioned bool          //nolint:unused
}

// Ensure GCPProvider implements the Providerer interface
var _ gcp_interface.GCPProviderer = &GCPProvider{}

func GetProvider(ctx context.Context) (common_interface.Providerer, error) {
	projectID := viper.GetString("gcp.project_id")
	if projectID == "" {
		return nil, fmt.Errorf("gcp.project_id is not set in configuration")
	}
	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is not set in configuration")
	}
	billingAccountID := viper.GetString("gcp.billing_account_id")
	if billingAccountID == "" {
		return nil, fmt.Errorf("gcp.billing_account_id is not set in configuration")
	}
	return NewGCPProvider(ctx, projectID, organizationID, billingAccountID)
}

// NewGCPProvider creates a new GCP provider
func NewGCPProvider(
	ctx context.Context,
	projectID, organizationID, billingAccountID string,
) (gcp_interface.GCPProviderer, error) {
	if projectID == "" {
		return nil, fmt.Errorf("gcp.project_id is required")
	}
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is required")
	}
	if billingAccountID == "" {
		return nil, fmt.Errorf("gcp.billing_account_id is required")
	}

	client, cleanup, err := NewGCPClient(
		ctx,
		organizationID,
	) // Implement this function to create a GCP client
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP client: %w", err)
	}

	provider := &GCPProvider{
		Client:           client,
		CleanupClient:    cleanup,
		ProjectID:        projectID,
		OrganizationID:   organizationID,
		BillingAccountID: billingAccountID,
		ClusterDeployer:  common.NewClusterDeployer(models.DeploymentTypeGCP),
		updateQueue:      make(chan display.UpdateAction, common.UpdateQueueSize),
	}

	if err := provider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize GCP provider: %w", err)
	}

	return provider, nil
}

// Initialize initializes the GCP provider
func (p *GCPProvider) Initialize(ctx context.Context) error {
	// Implement initialization logic here
	return nil
}

// loadDeploymentFromConfig loads deployment data from the configuration
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

// GetGCPClient returns the current GCP client
func (p *GCPProvider) GetGCPClient() gcp_interface.GCPClienter {
	return p.Client
}

// SetGCPClient sets a new GCP client
func (p *GCPProvider) SetGCPClient(client gcp_interface.GCPClienter) {
	p.Client = client
}

// EnsureProject ensures that a GCP project exists, creating it if necessary
func (p *GCPProvider) EnsureProject(
	ctx context.Context,
	projectID string,
) (string, error) {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return "", fmt.Errorf("global model or deployment is nil")
	}

	for _, machine := range m.Deployment.Machines {
		machine.SetMachineResourceState(
			models.GCPResourceTypeProject.ResourceString,
			models.ResourceStatePending,
		)
	}

	// Create the project
	createdProjectID, err := p.Client.EnsureProject(ctx, projectID)
	if err != nil {
		for _, machine := range m.Deployment.Machines {
			machine.SetMachineResourceState(
				models.GCPResourceTypeProject.ResourceString,
				models.ResourceStateFailed,
			)
		}
		return "", err
	}

	for _, machine := range m.Deployment.Machines {
		machine.SetMachineResourceState(
			models.GCPResourceTypeProject.ResourceString,
			models.ResourceStateSucceeded,
		)
	}

	return createdProjectID, nil
}

func (p *GCPProvider) DestroyResources(
	ctx context.Context,
	projectID string,
) error {
	return p.DestroyProject(ctx, projectID)
}

// DestroyProject destroys a specified GCP project
func (p *GCPProvider) DestroyProject(
	ctx context.Context,
	projectID string,
) error {
	return p.Client.DestroyProject(ctx, projectID)
}

// ListProjects lists all GCP projects
func (p *GCPProvider) ListProjects(
	ctx context.Context,
) ([]*resourcemanagerpb.Project, error) {
	return p.Client.ListProjects(ctx, &resourcemanagerpb.ListProjectsRequest{})
}

// PrepareDeployment prepares the deployment configuration
func (p *GCPProvider) PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	deployment, err := common.PrepareDeployment(ctx, models.DeploymentTypeGCP)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}
	deployment.GCP.Region = viper.GetString("gcp.region")
	deployment.GCP.Zone = viper.GetString("gcp.zone")
	deployment.GCP.BillingAccountID = viper.GetString("gcp.billing_account_id")
	deployment.GCP.OrganizationID = viper.GetString("gcp.organization_id")

	return deployment, nil
}

func (p *GCPProvider) StartResourcePolling(ctx context.Context) error {
	return p.StartResourcePolling(ctx)
}

// StartResourcePolling starts polling resources for updates
func (p *GCPProvider) PollResources(ctx context.Context) ([]interface{}, error) {
	if os.Getenv("ANDAIME_TEST_MODE") == "true" {
		// Skip display updates in test mode
		return []interface{}{}, nil
	}
	l := logger.Get()
	resources, err := p.Client.ListAllAssetsInProject(ctx, p.ProjectID)
	if err != nil {
		l.Errorf("Failed to start resource polling: %v", err)
		return nil, err
	}

	interfaceResources := make([]interface{}, len(resources))
	for i, r := range resources {
		interfaceResources[i] = r
	}
	return interfaceResources, nil
}

// FinalizeDeployment finalizes the deployment process
func (p *GCPProvider) FinalizeDeployment(ctx context.Context) error {
	l := logger.Get()
	l.Debug("Finalizing deployment... nothing to do.")
	return nil
}

// ListAllAssetsInProject lists all assets within a GCP project
func (p *GCPProvider) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	return p.Client.ListAllAssetsInProject(ctx, projectID)
}

// CheckAuthentication verifies GCP authentication
func (p *GCPProvider) CheckAuthentication(ctx context.Context) error {
	return p.Client.CheckAuthentication(ctx)
}

// EnableRequiredAPIs enables all required GCP APIs for the project
func (p *GCPProvider) EnableRequiredAPIs(ctx context.Context) error {
	m := display.GetGlobalModelFunc()

	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID
	if projectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	var apiEg errgroup.Group
	for _, api := range GetRequiredAPIs() {
		api := api
		apiEg.Go(func() error {
			for _, machine := range m.Deployment.Machines {
				machine.SetMachineResourceState(api, models.ResourceStatePending)
			}
			err := p.Client.EnableAPI(ctx, projectID, api)
			if err != nil {
				for _, machine := range m.Deployment.Machines {
					machine.SetMachineResourceState(api, models.ResourceStateFailed)
				}
				return fmt.Errorf("failed to enable API %s: %v", api, err)
			}
			for _, machine := range m.Deployment.Machines {
				machine.SetMachineResourceState(api, models.ResourceStateSucceeded)
			}
			return nil
		})
	}
	if err := apiEg.Wait(); err != nil {
		return fmt.Errorf("failed to enable APIs: %v", err)
	}

	return nil
}

// EnableAPI enables a specific GCP API for the project
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
		return fmt.Errorf("failed to check API status: %v", err)
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

// CreateVPCNetwork creates a VPC network in the specified project
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
				l.Infof("Compute Engine API is not yet active. Retrying... (VPC)")
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

// CreateFirewallRules creates firewall rules for a specified network
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

// CreateStorageBucket creates a storage bucket in GCP
func (p *GCPProvider) CreateStorageBucket(
	ctx context.Context,
	bucketName string,
) error {
	return p.Client.CreateStorageBucket(ctx, bucketName)
}

// ListBillingAccounts lists all billing accounts associated with the GCP organization
func (p *GCPProvider) ListBillingAccounts(ctx context.Context) ([]string, error) {
	return p.Client.ListBillingAccounts(ctx)
}

// SetBillingAccount sets the billing account for the GCP project
func (p *GCPProvider) SetBillingAccount(
	ctx context.Context,
	billingAccountID string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	l.Infof(
		"Setting billing account to %s for project %s",
		m.Deployment.GCP.BillingAccountID,
		m.Deployment.ProjectID,
	)

	return p.Client.SetBillingAccount(
		ctx,
		m.Deployment.GCP.BillingAccountID,
	)
}

// GetGCPClient initializes and returns a singleton GCP client
var (
	gcpClientInstance gcp_interface.GCPClienter
	gcpClientOnce     sync.Once
)

func GetGCPClient(
	ctx context.Context,
	organizationID string,
) (gcp_interface.GCPClienter, func(), error) {
	var err error
	var cleanup func()
	gcpClientOnce.Do(func() {
		gcpClientInstance, cleanup, err = NewGCPClientFunc(ctx, organizationID)
	})
	return gcpClientInstance, cleanup, err
}

// GetVMExternalIP retrieves the external IP of a VM instance
func (p *GCPProvider) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	return p.Client.GetVMExternalIP(ctx, vmName, locationData)
}

// GCPVMConfig holds the configuration for a GCP VM
type GCPVMConfig struct {
	ProjectID         string
	Region            string
	Zone              string
	VMName            string
	MachineType       string
	SSHUser           string
	PublicKeyMaterial string
}

// createNewGCPProject creates a new GCP project under the specified organization
func createNewGCPProject(ctx context.Context, organizationID string) (string, error) {
	projectID := fmt.Sprintf("andaime-project-%s", time.Now().Format("20060102150405"))

	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create resource manager client: %w", err)
	}
	defer client.Close()

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: &resourcemanagerpb.Project{
			ProjectId: projectID,
			Name:      "Andaime Project",
			Parent:    fmt.Sprintf("organizations/%s", organizationID),
		},
	}

	op, err := client.CreateProject(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create project: %w", err)
	}

	project, err := op.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to wait for project creation: %w", err)
	}

	return project.ProjectId, nil
}

func (p *GCPProvider) EnsureVPCNetwork(
	ctx context.Context,
	vpcNetworkName string,
) error {
	return p.Client.EnsureVPCNetwork(ctx, vpcNetworkName)
}

// EnsureFirewallRules ensures that firewall rules are set for a network
func (p *GCPProvider) EnsureFirewallRules(
	ctx context.Context,
	networkName string,
) error {
	return p.Client.EnsureFirewallRules(ctx, networkName)
}

// GetClusterDeployer returns the current ClusterDeployer
func (p *GCPProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return p.ClusterDeployer
}

// SetClusterDeployer sets a new ClusterDeployer
func (p *GCPProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.ClusterDeployer = deployer
}

// Creates the VM and returns the public and private IP addresses
func (p *GCPProvider) CreateVM(
	ctx context.Context,
	vmName string,
) (string, string, error) {
	instance, err := p.Client.CreateVM(ctx, vmName)
	if err != nil {
		return "", "", fmt.Errorf("failed to create VM: %w", err)
	}

	var publicIP string
	var privateIP string

	if len(instance.NetworkInterfaces) > 0 && len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
		publicIP = *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
	} else {
		return "", "", fmt.Errorf("no access configs found for instance %s - could not get public IP", vmName)
	}

	if len(instance.NetworkInterfaces) > 0 && instance.NetworkInterfaces[0].NetworkIP != nil {
		privateIP = *instance.NetworkInterfaces[0].NetworkIP
	} else {
		return "", "", fmt.Errorf("no network interface found for instance %s - could not get private IP", vmName)
	}

	return publicIP, privateIP, nil
}
