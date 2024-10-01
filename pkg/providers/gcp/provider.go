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
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

var NewGCPProviderFunc = NewGCPProviderFactory

// NewGCPProviderFactory is the factory function for GCPProvider.
func NewGCPProviderFactory(
	ctx context.Context,
	organizationID, billingAccountID string,
) (*GCPProvider, error) {
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is not set in configuration")
	}
	if billingAccountID == "" {
		return nil, fmt.Errorf("gcp.billing_account_id is not set in configuration")
	}

	client, cleanup, err := NewGCPClientFunc(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP client: %w", err)
	}

	provider, err := NewGCPProvider(ctx, organizationID, billingAccountID)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create GCP provider: %w", err)
	}

	provider.SetGCPClient(client)
	provider.CleanupClient = cleanup
	return provider, nil
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

// NewGCPProvider creates a new GCP provider
func NewGCPProvider(
	ctx context.Context,
	organizationID, billingAccountID string,
) (*GCPProvider, error) {
	gcpProvider := &GCPProvider{
		OrganizationID:   organizationID,
		BillingAccountID: billingAccountID,
		ClusterDeployer:  common.NewClusterDeployer(models.DeploymentTypeGCP),
		updateQueue:      make(chan display.UpdateAction, common.UpdateQueueSize),
	}

	if err := gcpProvider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize GCP provider: %w", err)
	}

	return gcpProvider, nil
}

// Initialize initializes the GCP provider
func (p *GCPProvider) Initialize(ctx context.Context) error {
	// Implement initialization logic here
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
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	for _, machine := range m.Deployment.Machines {
		machine.SetMachineResourceState(
			models.GCPResourceTypeProject.ResourceString,
			models.ResourceStatePending,
		)
	}

	// Create the project
	createdProjectID, err := p.GetGCPClient().
		EnsureProject(ctx, p.OrganizationID, m.Deployment.GetProjectID(), p.BillingAccountID)
	if err != nil {
		for _, machine := range m.Deployment.Machines {
			machine.SetMachineResourceState(
				models.GCPResourceTypeProject.ResourceString,
				models.ResourceStateFailed,
			)
		}
		return err
	}

	for _, machine := range m.Deployment.Machines {
		machine.SetMachineResourceState(
			models.GCPResourceTypeProject.ResourceString,
			models.ResourceStateSucceeded,
		)
	}

	m.Deployment.SetProjectID(createdProjectID)
	return nil
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
	return p.GetGCPClient().DestroyProject(ctx, projectID)
}

// ListProjects lists all GCP projects
func (p *GCPProvider) ListProjects(
	ctx context.Context,
) ([]*resourcemanagerpb.Project, error) {
	return p.GetGCPClient().ListProjects(ctx, &resourcemanagerpb.ListProjectsRequest{})
}

// PrepareDeployment prepares the deployment configuration
func (p *GCPProvider) PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	deployment, err := common.PrepareDeployment(ctx, models.DeploymentTypeGCP)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	return deployment, nil
}

// PollResources polls resources for updates
func (p *GCPProvider) PollResources(ctx context.Context) ([]interface{}, error) {
	if os.Getenv("ANDAIME_TEST_MODE") == "true" {
		// Skip display updates in test mode
		return []interface{}{}, nil
	}
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil || m.Deployment.GCP == nil ||
		m.Deployment.GCP.ProjectID == "" {
		l.Debug("Project ID not set yet, skipping resource polling")
		return nil, nil
	}

	// Check if the project exists
	projectExists, err := p.GetGCPClient().ProjectExists(ctx, m.Deployment.GCP.ProjectID)
	if err != nil {
		l.Errorf("Failed to check if project exists: %v", err)
		return nil, err
	}
	if !projectExists {
		l.Debugf("Project %s does not exist, skipping resource polling", m.Deployment.GCP.ProjectID)
		return nil, nil
	}

	resources, err := p.GetGCPClient().ListAllAssetsInProject(ctx, m.Deployment.GCP.ProjectID)
	if err != nil {
		l.Errorf("Failed to poll resources: %v", err)
		return nil, err
	}

	interfaceResources := make([]interface{}, len(resources))
	for i, r := range resources {
		interfaceResources[i] = r
	}
	return interfaceResources, nil
}

// StartResourcePolling starts polling resources for updates
func (p *GCPProvider) StartResourcePolling(ctx context.Context) <-chan error {
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)

		// Wait for the project ID to be set
		for {
			if m := display.GetGlobalModelFunc(); m != nil && m.Deployment != nil &&
				m.Deployment.GCP != nil &&
				m.Deployment.GCP.ProjectID != "" {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				// Wait and check again
			}
		}

		ticker := time.NewTicker(10 * time.Second) // Poll every 10 seconds
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := p.PollResources(ctx)
				if err != nil {
					errChan <- fmt.Errorf("failed to poll resources: %w", err)
				}
			}
		}
	}()
	return errChan
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
	return p.GetGCPClient().ListAllAssetsInProject(ctx, projectID)
}

// CheckAuthentication verifies GCP authentication
func (p *GCPProvider) CheckAuthentication(ctx context.Context) error {
	return p.GetGCPClient().CheckAuthentication(ctx)
}

// EnableRequiredAPIs enables all required GCP APIs for the project
func (p *GCPProvider) EnableRequiredAPIs(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.GetProjectID()
	if projectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	l.Info(fmt.Sprintf("Enabling required APIs for project %s", projectID))

	var apiEg errgroup.Group
	for _, api := range GetRequiredAPIs() {
		api := api
		apiEg.Go(func() error {
			for _, machine := range m.Deployment.Machines {
				machine.SetMachineResourceState(api, models.ResourceStatePending)
			}

			// Use backoff for enabling each API
			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = 5 * time.Minute

			err := backoff.Retry(func() error {
				err := p.GetGCPClient().EnableAPI(ctx, projectID, api)
				if err != nil {
					l.Warn(fmt.Sprintf("Failed to enable API %s, retrying: %v", api, err))
					return err
				}
				return nil
			}, b)

			if err != nil {
				for _, machine := range m.Deployment.Machines {
					machine.SetMachineResourceState(api, models.ResourceStateFailed)
				}
				return fmt.Errorf("failed to enable API %s after retries: %v", api, err)
			}

			for _, machine := range m.Deployment.Machines {
				machine.SetMachineResourceState(api, models.ResourceStateSucceeded)
			}
			l.Info(fmt.Sprintf("Successfully enabled API: %s", api))
			return nil
		})
	}
	if err := apiEg.Wait(); err != nil {
		return fmt.Errorf("failed to enable APIs: %v", err)
	}

	l.Info("All required APIs enabled successfully")
	return nil
}

// EnableAPI enables a specific GCP API for the project
func (p *GCPProvider) EnableAPI(ctx context.Context, apiName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.GetProjectID()
	if projectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	l.Infof("Checking API status: %s for project: %s", apiName, projectID)

	// First, check if the API is already enabled
	enabled, err := p.GetGCPClient().IsAPIEnabled(ctx, projectID, apiName)
	if err != nil {
		l.Warnf("Failed to check API status: %v", err)
		return fmt.Errorf("failed to check API status: %v", err)
	} else if enabled {
		l.Infof("API %s is already enabled", apiName)
		return nil
	}

	l.Infof("Attempting to enable API: %s for project: %s", apiName, projectID)

	err = p.GetGCPClient().EnableAPI(ctx, projectID, apiName)
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
	l.Infof("Creating VPC network %s in project %s", networkName, m.Deployment.GetProjectID())

	// First, ensure that the Compute Engine API is enabled
	err := p.EnableAPI(ctx, "compute.googleapis.com")
	if err != nil {
		return fmt.Errorf("failed to enable Compute Engine API: %w", err)
	}

	// Define the exponential backoff strategy
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 3 * time.Minute

	// Define the operation to retry
	operation := func() error {
		l.Infof("Attempting to create VPC network %s...", networkName)
		err := p.GetGCPClient().CreateVPCNetwork(ctx, networkName)
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

	err := p.GetGCPClient().CreateFirewallRules(ctx, networkName)
	if err != nil {
		return fmt.Errorf("failed to create firewall rules: %v", err)
	}

	l.Infof("Firewall rules created successfully for network: %s", networkName)
	return nil
}

// // CreateStorageBucket creates a storage bucket in GCP
// func (p *GCPProvider) CreateStorageBucket(
// 	ctx context.Context,
// 	bucketName string,
// ) error {
// 	return p.GetGCPClient().CreateStorageBucket(ctx, bucketName)
// }

// ListBillingAccounts lists all billing accounts associated with the GCP organization
func (p *GCPProvider) ListBillingAccounts(ctx context.Context) ([]string, error) {
	return p.GetGCPClient().ListBillingAccounts(ctx)
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
		m.Deployment.GetProjectID(),
	)

	return p.GetGCPClient().SetBillingAccount(
		ctx,
		m.Deployment.GetProjectID(),
		billingAccountID,
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
	return p.GetGCPClient().GetVMExternalIP(ctx, vmName, locationData)
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
	return p.GetGCPClient().EnsureVPCNetwork(ctx, vpcNetworkName)
}

// EnsureFirewallRules ensures that firewall rules are set for a network
func (p *GCPProvider) EnsureFirewallRules(
	ctx context.Context,
	networkName string,
) error {
	return p.GetGCPClient().EnsureFirewallRules(ctx, networkName)
}

// GetClusterDeployer returns the current ClusterDeployer
func (p *GCPProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return p.ClusterDeployer
}

// SetClusterDeployer sets a new ClusterDeployer
func (p *GCPProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.ClusterDeployer = deployer
}

// CheckPermissions checks the current user's permissions
func (p *GCPProvider) CheckPermissions(ctx context.Context) error {
	return p.GetGCPClient().CheckPermissions(ctx)
}

// Creates the VM and returns the public and private IP addresses
func (p *GCPProvider) CreateVM(
	ctx context.Context,
	vmName string,
) (string, string, error) {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return "", "", fmt.Errorf("global model or deployment is nil")
	}

	instance, err := p.GetGCPClient().
		CreateVM(ctx, m.Deployment.GetProjectID(), m.Deployment.Machines[vmName])
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
