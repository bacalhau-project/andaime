// pkg/providers/azure/provider.go
package azure

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/goroutine"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/azure"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

func init() {
	logger.InitProduction()
}

func NewAzureProviderFactory(ctx context.Context) (*AzureProvider, error) {
	subscriptionID := viper.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure.subscription_id is not set in configuration")
	}

	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	provider, err := NewAzureProvider(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure provider: %w", err)
	}

	provider.SetAzureClient(client)
	return provider, nil
}

// Constants related to Azure Provider configurations.
const (
	UpdateQueueSize         = 100
	ResourcePollingInterval = 2 * time.Second
	DebugFilePath           = "/tmp/andaime-debug.log"
	DebugFilePermissions    = 0644
	WaitingForMachinesTime  = 1 * time.Minute
	DefaultSSHUser          = "azureuser"
	DefaultSSHPort          = 22
)

// AzureProvider implements the Providerer interface.
type AzureProvider struct {
	SubscriptionID      string
	ResourceGroupName   string
	Tags                map[string]*string
	Client              azure_interface.AzureClienter
	ClusterDeployer     common_interface.ClusterDeployerer
	SSHClient           sshutils.SSHClienter
	SSHUser             string
	SSHPort             int
	lastResourceQuery   time.Time
	cachedResources     []interface{}
	serviceMutex        sync.Mutex //nolint:unused
	servicesProvisioned bool
	UpdateQueue         chan display.UpdateAction
	UpdateMutex         sync.Mutex
}

func (p *AzureProvider) GetAzureClient() azure_interface.AzureClienter {
	return p.Client
}

func (p *AzureProvider) SetAzureClient(client azure_interface.AzureClienter) {
	p.Client = client
}

var NewAzureProviderFunc = NewAzureProvider

// NewAzureProvider creates and initializes a new AzureProvider instance with subscriptionID.
func NewAzureProvider(
	ctx context.Context,
	subscriptionID string,
) (*AzureProvider, error) {
	// Create the AzureProvider instance.
	provider := &AzureProvider{
		SubscriptionID:  subscriptionID,
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAzure),
	}

	// Initialize the provider (e.g., setup SSH keys, start update processor).
	if err := provider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	if provider.SubscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is not set in configuration")
	}

	if provider.ResourceGroupName == "" {
		return nil, fmt.Errorf("resource group name is not set in configuration")
	}

	if provider.Tags == nil {
		return nil, fmt.Errorf("tags are not set in configuration")
	}

	// Initialize the Azure client
	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}
	provider.Client = client

	return provider, nil
}

func (p *AzureProvider) CheckAuthentication(ctx context.Context) error {
	// Attempt to list resource groups as a simple authentication check
	_, err := p.Client.ListAllResourceGroups(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

// Initialize sets up the AzureProvider by configuring SSH keys and starting the update processor.
func (p *AzureProvider) Initialize(ctx context.Context) error {
	// Retrieve SSH key paths from configuration.
	sshPublicKeyPath := viper.GetString("general.ssh_public_key_path")
	sshPrivateKeyPath := viper.GetString("general.ssh_private_key_path")

	// Validate SSH key paths.
	if sshPublicKeyPath == "" {
		return fmt.Errorf("general.ssh_public_key_path is required")
	}
	if sshPrivateKeyPath == "" {
		return fmt.Errorf("general.ssh_private_key_path is required")
	}

	// Expand the SSH key paths to absolute paths.
	expandedPublicKeyPath, err := homedir.Expand(sshPublicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand public key path: %w", err)
	}
	expandedPrivateKeyPath, err := homedir.Expand(sshPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand private key path: %w", err)
	}

	// Verify that SSH key files exist.
	if _, err := os.Stat(expandedPublicKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH public key file does not exist: %s", expandedPublicKeyPath)
	}
	if _, err := os.Stat(expandedPrivateKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH private key file does not exist: %s", expandedPrivateKeyPath)
	}

	// Update the configuration with the expanded SSH key paths.
	viper.Set("general.ssh_public_key_path", expandedPublicKeyPath)
	viper.Set("general.ssh_private_key_path", expandedPrivateKeyPath)

	// Set SSH user with a default fallback.
	p.SSHUser = viper.GetString("general.ssh_user")
	if p.SSHUser == "" {
		p.SSHUser = DefaultSSHUser // Default SSH user for Azure VMs.
	}

	// Set SSH port with a default fallback.
	p.SSHPort = viper.GetInt("general.ssh_port")
	if p.SSHPort == 0 {
		p.SSHPort = DefaultSSHPort // Default SSH port.
	}

	p.ResourceGroupName = "andaime-rg" + "-" + time.Now().Format("20060102150405")

	p.Tags = GenerateTags(
		viper.GetString("general.project_prefix"),
		viper.GetString("general.unique_id"),
	)

	// Start the update processor goroutine.
	go display.GetGlobalModelFunc().StartUpdateProcessor(ctx)

	return nil
}

// GetClusterDeployer returns the current ClusterDeployer.
func (p *AzureProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return p.ClusterDeployer
}

// SetClusterDeployer sets a new ClusterDeployer.
func (p *AzureProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.ClusterDeployer = deployer
}

// GetOrCreateResourceGroup retrieves an existing resource group or creates a new one if it doesn't exist.
func (p *AzureProvider) GetOrCreateResourceGroup(
	ctx context.Context,
	resourceGroupName string,
	locationData string,
	tags map[string]string,
) (*armresources.ResourceGroup, error) {
	resourceGroup, err := p.Client.GetOrCreateResourceGroup(
		ctx,
		resourceGroupName,
		locationData,
		tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create resource group: %w", err)
	}
	return resourceGroup, nil
}

// DestroyResourceGroup deletes the specified resource group.
func (p *AzureProvider) DestroyResourceGroup(ctx context.Context, resourceGroupName string) error {
	return p.Client.DestroyResourceGroup(ctx, resourceGroupName)
}

// DestroyResources destroys the specified resource group if it exists.
func (p *AzureProvider) DestroyResources(ctx context.Context, resourceGroupName string) error {
	l := logger.Get()
	client := p.Client

	// Check if the resource group exists before attempting to destroy it.
	exists, err := client.ResourceGroupExists(ctx, resourceGroupName)
	if err != nil {
		l.Errorf("Error checking if resource group exists: %v", err)
		return err
	}

	if !exists {
		l.Infof("Resource group %s does not exist, skipping destruction", resourceGroupName)
		return nil
	}

	l.Infof("Destroying resource group %s", resourceGroupName)
	err = client.DestroyResourceGroup(ctx, resourceGroupName)
	if err != nil {
		l.Errorf("Error destroying resource group %s: %v", resourceGroupName, err)
		return err
	}

	l.Infof("Resource group %s destroyed successfully", resourceGroupName)
	return nil
}

// GetVMExternalIP retrieves the external IP of a VM instance.
func (p *AzureProvider) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	if locationData["location"] == "" {
		return "", fmt.Errorf("location data is empty")
	}
	return p.Client.GetVMExternalIP(ctx, vmName, locationData)
}

// StartResourcePolling starts polling Azure resources for updates.
func (p *AzureProvider) StartResourcePolling(ctx context.Context) error {
	l := logger.Get()
	writeToDebugLog("Starting StartResourcePolling")

	if os.Getenv("ANDAIME_TEST_MODE") == "true" {
		l.Debug("ANDAIME_TEST_MODE is set to true, skipping resource polling")
		return nil
	}

	resourceTicker := time.NewTicker(ResourcePollingInterval)
	defer resourceTicker.Stop()

	quit := make(chan struct{})
	goroutineID := goroutine.RegisterGoroutine("AzureResourcePolling")
	defer goroutine.DeregisterGoroutine(goroutineID)

	go func() {
		<-ctx.Done()
		close(quit)
	}()

	defer func() {
		if r := recover(); r != nil {
			l.Errorf("Recovered from panic in StartResourcePolling: %v", r)
			writeToDebugLog(fmt.Sprintf("Panic in StartResourcePolling: %v", r))
			debug.PrintStack()
		}
		writeToDebugLog("Exiting StartResourcePolling function")
	}()

	pollCount := 0
	for {
		select {
		case <-resourceTicker.C:
			m := display.GetGlobalModelFunc()
			if m.Quitting {
				writeToDebugLog("Quitting detected, stopping resource polling")
				return nil
			}
			pollCount++
			start := time.Now()
			writeToDebugLog(fmt.Sprintf("Starting poll #%d", pollCount))

			resources, err := p.PollResources(ctx)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
				writeToDebugLog(fmt.Sprintf("Failed to poll and update resources: %v", err))
				p.CancelAllDeployments(ctx)
				return err
			}

			writeToDebugLog(fmt.Sprintf("Poll #%d: Found %d resources", pollCount, len(resources)))

			allResourcesProvisioned := true
			for _, resource := range resources {
				resourceMap, ok := resource.(map[string]interface{})
				if !ok {
					l.Warn("Invalid resource format")
					continue
				}
				provisioningState, ok := resourceMap["provisioningState"].(string)
				if !ok {
					l.Warn("Provisioning state not found or invalid")
					continue
				}
				writeToDebugLog(
					fmt.Sprintf(
						"Resource: %s - Provisioning State: %s",
						resourceMap["name"],
						provisioningState,
					),
				)
				if provisioningState != "Succeeded" {
					allResourcesProvisioned = false
				}
			}

			elapsed := time.Since(start)
			writeToDebugLog(fmt.Sprintf("PollResources #%d took %v", pollCount, elapsed))

			p.logDeploymentStatus()

			if allResourcesProvisioned && p.AllMachinesComplete() {
				writeToDebugLog(
					"All resources provisioned and machines completed, stopping resource polling",
				)

				// Just for visual, set all the machines to complete
				for _, machine := range m.Deployment.Machines {
					allMachineResources := machine.GetMachineResources()
					for _, resource := range allMachineResources {
						machine.SetMachineResourceState(
							resource.ResourceName,
							models.ResourceStateSucceeded,
						)
					}
				}
				return nil
			}
		case <-quit:
			l.Debug("Quit signal received, exiting resource polling")
			writeToDebugLog("Quit signal received, exiting resource polling")
			return nil
		}
	}
}

// logDeploymentStatus logs the current status of the deployment.
func (p *AzureProvider) logDeploymentStatus() {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m.Deployment == nil {
		l.Debug("Deployment is nil")
		writeToDebugLog("Deployment is nil")
		return
	}

	writeToDebugLog(
		fmt.Sprintf(
			"Deployment Status - Name: %s, ResourceGroup: %s",
			m.Deployment.Name,
			m.Deployment.Azure.ResourceGroupName,
		),
	)
	writeToDebugLog(fmt.Sprintf("Total Machines: %d", len(m.Deployment.Machines)))

	for _, machine := range m.Deployment.Machines {
		writeToDebugLog(
			fmt.Sprintf(
				"Machine Name: %s, PublicIP: %s, PrivateIP: %s",
				machine.GetName(),
				machine.GetPublicIP(),
				machine.GetPrivateIP(),
			),
		)
		writeToDebugLog(
			fmt.Sprintf("Machine %s - Docker: %v, CorePackages: %v, Bacalhau: %v, SSH: %v",
				machine.GetName(),
				machine.GetServiceState("Docker"),
				machine.GetServiceState("CorePackages"),
				machine.GetServiceState("Bacalhau"),
				machine.GetServiceState("SSH"),
			),
		)

		completedResources, totalResources := machine.ResourcesComplete()
		writeToDebugLog(
			fmt.Sprintf(
				"Machine %s - Resources: %d/%d complete",
				machine.GetName(),
				completedResources,
				totalResources,
			),
		)

		if machine.GetMachineResources() != nil {
			for resourceType, resource := range machine.GetMachineResources() {
				writeToDebugLog(fmt.Sprintf("Machine %s - Resource %s: State: %v, Value: %s",
					machine.GetName(),
					resourceType,
					resource.ResourceState,
					resource.ResourceValue,
				))
			}
		} else {
			writeToDebugLog(fmt.Sprintf("Machine %s - No machine resources", machine.GetName()))
		}
	}
}

func (p *AzureProvider) GetResources(
	ctx context.Context,
	resourceGroupName string,
	tags map[string]*string,
) ([]interface{}, error) {
	return p.Client.GetResources(ctx, p.SubscriptionID, resourceGroupName, tags)
}

// writeToDebugLog writes a message to the debug log file with a timestamp.
func writeToDebugLog(message string) {
	debugFilePath := common.DebugFilePath
	debugFile, err := os.OpenFile(
		debugFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		DebugFilePermissions, //nolint:mnd
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening debug log file %s: %v\n", debugFilePath, err)
		return
	}
	defer debugFile.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s\n", timestamp, message)
	if _, err := debugFile.WriteString(logMessage); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to debug log file: %v\n", err)
	}
}

// CancelAllDeployments cancels all ongoing deployments.
func (p *AzureProvider) CancelAllDeployments(ctx context.Context) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	l.Info("Cancelling all deployments")
	writeToDebugLog("Cancelling all deployments")

	for _, machine := range m.Deployment.Machines {
		if !machine.IsComplete() {
			machine.SetComplete()
		}
	}

	// Cancel the context to stop all ongoing operations.
	if cancelFunc, ok := ctx.Value("cancelFunc").(context.CancelFunc); ok {
		cancelFunc()
	}
}

// AllMachinesComplete checks if all machines in the deployment are marked as complete.
func (p *AzureProvider) AllMachinesComplete() bool {
	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		if !machine.IsComplete() {
			return false
		}
	}
	return true
}

func (p *AzureProvider) GetSKUsByLocation(
	ctx context.Context,
	location string,
) ([]armcompute.ResourceSKU, error) {
	return p.Client.GetSKUsByLocation(ctx, location)
}

func (p *AzureProvider) ListAllResourceGroups(
	ctx context.Context,
) ([]*armresources.ResourceGroup, error) {
	if p.Client == nil {
		l := logger.Get()
		l.Error("Azure client is not initialized")
		return nil, fmt.Errorf("Azure client is not initialized")
	}
	return p.Client.ListAllResourceGroups(ctx)
}

func (p *AzureProvider) ListAllResourcesInSubscription(
	ctx context.Context,
	tags map[string]*string,
) ([]interface{}, error) {
	return p.Client.ListAllResourcesInSubscription(ctx, p.SubscriptionID, p.Tags)
}

// Add this method to the AzureProvider struct

func (p *AzureProvider) ValidateMachineType(
	ctx context.Context,
	location, machineType string,
) (bool, error) {
	return p.Client.ValidateMachineType(ctx, location, machineType)
}

func (p *AzureProvider) FinalizeDeployment(ctx context.Context) error {
	return nil
}
