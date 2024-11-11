// pkg/providers/gcp/provider.go
package gcp

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/asset/apiv1/assetpb"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
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
	NetworkName         string
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

	for _, machine := range m.Deployment.GetMachines() {
		machine.SetMachineResourceState(
			models.GCPResourceTypeProject.ResourceString,
			models.ResourceStatePending,
		)
	}

	// Create the project
	createdProjectID, err := p.GetGCPClient().
		EnsureProject(ctx, p.OrganizationID, p.ProjectID, p.BillingAccountID)
	if err != nil {
		for _, machine := range m.Deployment.GetMachines() {
			machine.SetMachineResourceState(
				models.GCPResourceTypeProject.ResourceString,
				models.ResourceStateFailed,
			)
		}
		return err
	}

	for _, machine := range m.Deployment.GetMachines() {
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
		p.ProjectID == "" {
		l.Debug("Project ID not set yet, skipping resource polling")
		return nil, nil
	}

	// Use backoff for retrying the polling operation
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 30 * time.Second
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 5 * time.Second
	b.Multiplier = 2
	b.RandomizationFactor = 0.1

	var resources []*assetpb.Asset
	operation := func() error {
		// Check if the project exists
		projectExists, err := p.GetGCPClient().ProjectExists(ctx, m.Deployment.GCP.ProjectID)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "deadline exceeded") ||
				strings.Contains(err.Error(), "connection reset") {
				l.Warnf("Temporary network error checking project: %v, retrying...", err)
				return err // Retryable error
			}
			return backoff.Permanent(fmt.Errorf("failed to check if project exists: %w", err))
		}
		if !projectExists {
			l.Debugf(
				"Project %s does not exist, skipping resource polling",
				m.Deployment.GCP.ProjectID,
			)
			return nil
		}

		// List all assets
		resources, err = p.GetGCPClient().ListAllAssetsInProject(ctx, m.Deployment.GCP.ProjectID)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "deadline exceeded") ||
				strings.Contains(err.Error(), "connection reset") {
				l.Warnf("Temporary network error polling resources: %v, retrying...", err)
				return err // Retryable error
			}
			return backoff.Permanent(fmt.Errorf("failed to poll resources: %w", err))
		}
		return nil
	}

	err := backoff.Retry(operation, b)
	if err != nil {
		l.Errorf("Failed to poll resources after retries: %v", err)
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
		l := logger.Get()

		// Wait for the project ID to be set with backoff
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 5 * time.Minute
		b.InitialInterval = 1 * time.Second
		b.MaxInterval = 10 * time.Second

		err := backoff.Retry(func() error {
			if m := display.GetGlobalModelFunc(); m != nil && m.Deployment != nil &&
				m.Deployment.GCP != nil &&
				m.Deployment.GCP.ProjectID != "" {
				return nil
			}
			return fmt.Errorf("waiting for project ID")
		}, b)

		if err != nil {
			errChan <- fmt.Errorf("timeout waiting for project ID: %w", err)
			return
		}

		ticker := time.NewTicker(10 * time.Second) //nolint:mnd
		defer ticker.Stop()

		consecutiveErrors := 0
		maxConsecutiveErrors := 5

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := p.PollResources(ctx)
				if err != nil {
					consecutiveErrors++
					l.Warnf("Poll error (%d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

					if consecutiveErrors >= maxConsecutiveErrors {
						errChan <- fmt.Errorf("polling failed after %d consecutive errors: %w",
							maxConsecutiveErrors, err)
						return
					}

					// Increase polling interval temporarily after errors
					ticker.Reset(30 * time.Second)
				} else {
					consecutiveErrors = 0
					// Reset polling interval on success
					ticker.Reset(10 * time.Second)
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
	return p.GetGCPClient().CheckAuthentication(ctx, p.ProjectID)
}

// EnableRequiredAPIs enables all required GCP APIs for the project
func (p *GCPProvider) EnableRequiredAPIs(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	if p.ProjectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	l.Info(fmt.Sprintf("Enabling required APIs for project %s", p.ProjectID))

	var apiEg errgroup.Group
	for _, api := range GetRequiredAPIs() {
		api := api
		apiEg.Go(func() error {
			for _, machine := range m.Deployment.GetMachines() {
				machine.SetMachineResourceState(api, models.ResourceStatePending)
			}

			// Use backoff for enabling each API
			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = 5 * time.Minute

			err := backoff.Retry(func() error {
				err := p.GetGCPClient().EnableAPI(ctx, p.ProjectID, api)
				if err != nil {
					l.Warn(fmt.Sprintf("Failed to enable API %s, retrying: %v", api, err))
					return err
				}
				return nil
			}, b)

			if err != nil {
				for _, machine := range m.Deployment.GetMachines() {
					machine.SetMachineResourceState(api, models.ResourceStateFailed)
				}
				return fmt.Errorf("failed to enable API %s after retries: %v", api, err)
			}

			for _, machine := range m.Deployment.GetMachines() {
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

	if p.ProjectID == "" {
		return fmt.Errorf("project ID is not set in the deployment")
	}

	l.Infof("Checking API status: %s for project: %s", apiName, p.ProjectID)

	// First, check if the API is already enabled
	enabled, err := p.GetGCPClient().IsAPIEnabled(ctx, p.ProjectID, apiName)
	if err != nil {
		l.Warnf("Failed to check API status: %v", err)
		return fmt.Errorf("failed to check API status: %v", err)
	} else if enabled {
		l.Infof("API %s is already enabled", apiName)
		return nil
	}

	l.Infof("Attempting to enable API: %s for project: %s", apiName, p.ProjectID)

	err = p.GetGCPClient().EnableAPI(ctx, p.ProjectID, apiName)
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
	l.Infof("Creating VPC network %s in project %s", networkName, p.ProjectID)

	for _, machine := range m.Deployment.GetMachines() {
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Creating VPC Network.",
			),
		)
	}

	// Execute the operation with backoff
	l.Infof("Attempting to create VPC network %s...", networkName)
	err := p.GetGCPClient().CreateVPCNetwork(ctx, p.ProjectID, networkName)
	if err != nil {
		l.Errorf("Failed to create VPC network after multiple attempts: %v", err)
		return fmt.Errorf("failed to create VPC network: %v", err)
	}

	for _, machine := range m.Deployment.GetMachines() {
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeInstance,
				models.ResourceStateSucceeded,
				"VPC Network Created.",
			),
		)
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

	err := p.GetGCPClient().CreateFirewallRules(ctx, p.ProjectID, networkName)
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
		p.ProjectID,
	)

	return p.GetGCPClient().SetBillingAccount(
		ctx,
		p.ProjectID,
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
	return p.GetGCPClient().EnsureVPCNetwork(ctx, p.ProjectID, vpcNetworkName)
}

// EnsureFirewallRules ensures that firewall rules are set for a network
func (p *GCPProvider) EnsureFirewallRules(
	ctx context.Context,
	projectID string,
	networkName string,
	allowedPorts []int,
) error {
	return p.GetGCPClient().EnsureFirewallRules(
		ctx,
		projectID,
		networkName,
		allowedPorts,
	)
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
	return p.GetGCPClient().CheckPermissions(ctx, p.ProjectID)
}

func (p *GCPProvider) CreateAndConfigureVM(
	ctx context.Context,
	machine models.Machiner,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	l.Infof("Starting VM creation process for %s in zone %s",
		machine.GetName(), machine.GetLocation())

	m.UpdateStatus(models.NewDisplayStatusWithText(
		machine.GetName(),
		models.GCPResourceTypeInstance,
		models.ResourceStatePending,
		"Creating VM",
	))

	// Get region from zone
	region := extractRegionFromZone(machine.GetLocation())
	l.Infof("Determined region %s for VM %s", region, machine.GetName())

	// Attempt IP allocation with retries
	publicIP, err := p.allocateIPWithRetries(ctx, machine.GetName(), region)
	if err != nil {
		return fmt.Errorf("failed to allocate IP for VM %s: %w", machine.GetName(), err)
	}
	l.Infof("Successfully allocated IP %s for VM %s", publicIP, machine.GetName())

	// Create the VM instance
	instance, err := p.GetGCPClient().CreateVM(
		ctx,
		p.ProjectID,
		machine,
		publicIP,
		p.NetworkName,
	)
	if err != nil {
		// Attempt to release the IP if VM creation fails
		if releaseErr := p.releaseIP(ctx, *publicIP.Address, region); releaseErr != nil {
			l.Warnf("Failed to release IP %s after VM creation failure: %v", publicIP, releaseErr)
		}
		return fmt.Errorf("failed to create VM instance: %w", err)
	}

	var publicIPAddress string
	var privateIPAddress string

	// This may be repetitive, but it's a sanity check by pulling the IP from the instance
	if len(instance.NetworkInterfaces) > 0 && len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
		publicIPAddress = *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
	} else {
		l.Errorf("No access configs found for instance %s - could not get public IP", machine.GetName())
		m.Deployment.Machines[machine.GetName()].SetFailed(true)
		return fmt.Errorf("no access configs found for instance %s - could not get public IP", machine.GetName())
	}

	if len(instance.NetworkInterfaces) > 0 && instance.NetworkInterfaces[0].NetworkIP != nil {
		privateIPAddress = *instance.NetworkInterfaces[0].NetworkIP
	} else {
		l.Errorf("No network interface found for instance %s - could not get private IP", machine.GetName())
		m.Deployment.Machines[machine.GetName()].SetFailed(true)
		return fmt.Errorf("no network interface found for instance %s - could not get private IP", machine.GetName())
	}

	machine.SetPublicIP(publicIPAddress)
	machine.SetPrivateIP(privateIPAddress)

	l.Infof("Successfully configured VM %s with public IP %s and private IP %s",
		machine.GetName(), publicIPAddress, privateIPAddress)

	return nil
}

func (p *GCPProvider) allocateIPWithRetries(
	ctx context.Context,
	vmName, region string,
) (*computepb.Address, error) {
	l := logger.Get()
	config := DefaultIPAllocationConfig

	var lastErr error
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		if attempt > 0 {
			l.Infof("Retrying IP allocation for VM %s (attempt %d/%d)",
				vmName, attempt+1, config.MaxRetries)
			time.Sleep(config.RetryInterval)
		}

		// Ensure projectID is set
		if p.ProjectID == "" {
			return nil, fmt.Errorf("projectID is not set in allocateIPWithRetries")
		}

		// Try to allocate a new IP
		ip, err := p.tryAllocateIP(ctx, vmName, region)
		if err == nil {
			return ip, nil
		}

		lastErr = err
		l.Warnf("IP allocation attempt %d failed for VM %s: %v",
			attempt+1, vmName, err)

		// If context is canceled, stop retrying
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context canceled during IP allocation: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("failed to allocate IP after %d attempts: %w",
		config.MaxRetries, lastErr)
}

func (p *GCPProvider) tryAllocateIP(
	ctx context.Context,
	vmName, region string,
) (*computepb.Address, error) {
	l := logger.Get()
	if p.ProjectID == "" {
		return nil, fmt.Errorf("projectID is not set in tryAllocateIP")
	}

	// First try to find an available IP in the project
	availableIP, err := p.findAvailableIP(ctx, region)
	if err == nil {
		l.Infof("Found available IP %s for VM %s", availableIP, vmName)
		return availableIP, nil
	}
	l.Infof("No available IPs found for VM %s, attempting to reserve new IP", vmName)

	// If no available IPs, create a new one
	ipName := fmt.Sprintf("%s-ip-%s", vmName, randomString(6))
	addressType := computepb.Address_EXTERNAL.String()

	address := &computepb.Address{
		Name:        &ipName,
		AddressType: &addressType,
	}

	addr, err := p.GetGCPClient().CreateIP(ctx, p.ProjectID, region, address)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve IP address: %w", err)
	}

	l.Infof("Successfully reserved new IP %s for VM %s", *addr.Address, vmName)
	return addr, nil
}

func (p *GCPProvider) findAvailableIP(
	ctx context.Context,
	region string,
) (*computepb.Address, error) {
	l := logger.Get()
	if p.ProjectID == "" {
		return nil, fmt.Errorf("projectID is not set in findAvailableIP")
	}

	// List all addresses in the region
	addressList, err := p.GetGCPClient().ListAddresses(ctx, p.ProjectID, region)
	if err != nil {
		return nil, fmt.Errorf("failed to list IP addresses: %w", err)
	}

	// Find an unassigned address
	for _, addr := range addressList {
		if *addr.Status == "RESERVED" && addr.Users == nil {
			l.Infof("Found unused IP address: %s", *addr.Address)
			return addr, nil
		}
	}

	return nil, fmt.Errorf("no available IP addresses found")
}

func (p *GCPProvider) releaseIP(ctx context.Context, ip, region string) error {
	l := logger.Get()
	l.Infof("Attempting to release IP %s in region %s", ip, region)

	// Find the address resource by IP
	addressList, err := p.GetGCPClient().ListAddresses(ctx, p.ProjectID, region)
	if err != nil {
		return fmt.Errorf("failed to list IP addresses: %w", err)
	}

	for _, addr := range addressList {
		if *addr.Address == ip {
			err := p.GetGCPClient().DeleteIP(ctx, p.ProjectID, region, *addr.Name)
			if err != nil {
				return fmt.Errorf("failed to delete IP address: %w", err)
			}
			l.Infof("Successfully released IP %s", ip)
			return nil
		}
	}

	return fmt.Errorf("IP address %s not found", ip)
}

func extractRegionFromZone(zone string) string {
	// Zones typically look like "us-central1-a", we want "us-central1"
	parts := strings.Split(zone, "-")
	if len(parts) < 3 {
		return zone // Return original if format is unexpected
	}
	return strings.Join(parts[:len(parts)-1], "-")
}

func randomString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
