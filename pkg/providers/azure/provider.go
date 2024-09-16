// pkg/providers/azure/provider.go
package azure

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/goroutine"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

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
	Client              AzureClienter
	Config              *viper.Viper
	ClusterDeployer     common.ClusterDeployerer
	SSHClient           sshutils.SSHClienter
	SSHUser             string
	SSHPort             int
	lastResourceQuery   time.Time
	cachedResources     []interface{}
	updateQueue         chan common.UpdateAction
	updateMutex         sync.Mutex
	updateProcessorDone chan struct{}
	serviceMutex        sync.Mutex //nolint:unused
	servicesProvisioned bool       //nolint:unused
}

// Ensure AzureProvider implements the Providerer interface.
var _ common.Providerer = &AzureProvider{}

// NewAzureProvider creates and initializes a new AzureProvider instance with subscriptionID.
func NewAzureProvider(ctx context.Context, subscriptionID string) (common.Providerer, error) {
	// Initialize the Azure client.
	client, err := NewAzureClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	// Create the AzureProvider instance.
	provider := &AzureProvider{
		Client:          client,
		Config:          viper.GetViper(),
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAzure),
		updateQueue:     make(chan common.UpdateAction, UpdateQueueSize),
	}

	// Initialize the provider (e.g., setup SSH keys, start update processor).
	if err := provider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	return provider, nil
}

func (p *AzureProvider) CheckAuthentication(ctx context.Context) error {
	// Attempt to list resource groups as a simple authentication check
	_, err := p.Client.ListResourceGroups(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

// Initialize sets up the AzureProvider by configuring SSH keys and starting the update processor.
func (p *AzureProvider) Initialize(ctx context.Context) error {
	// Retrieve SSH key paths from configuration.
	sshPublicKeyPath := p.Config.GetString("general.ssh_public_key_path")
	sshPrivateKeyPath := p.Config.GetString("general.ssh_private_key_path")

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
	p.Config.Set("general.ssh_public_key_path", expandedPublicKeyPath)
	p.Config.Set("general.ssh_private_key_path", expandedPrivateKeyPath)

	// Set SSH user with a default fallback.
	p.SSHUser = p.Config.GetString("general.ssh_user")
	if p.SSHUser == "" {
		p.SSHUser = DefaultSSHUser // Default SSH user for Azure VMs.
	}

	// Set SSH port with a default fallback.
	p.SSHPort = p.Config.GetInt("general.ssh_port")
	if p.SSHPort == 0 {
		p.SSHPort = DefaultSSHPort // Default SSH port.
	}

	// Initialize the SSH client if needed (placeholder).
	// p.SSHClient = sshutils.NewSSHClient(...)

	// Initialize the update queue and start the update processor goroutine.
	p.updateQueue = make(chan common.UpdateAction, UpdateQueueSize)
	go p.startUpdateProcessor(ctx)

	return nil
}

// GetClusterDeployer returns the current ClusterDeployer.
func (p *AzureProvider) GetClusterDeployer() common.ClusterDeployerer {
	return p.ClusterDeployer
}

// SetClusterDeployer sets a new ClusterDeployer.
func (p *AzureProvider) SetClusterDeployer(deployer common.ClusterDeployerer) {
	p.ClusterDeployer = deployer
}

// GetOrCreateResourceGroup retrieves an existing resource group or creates a new one if it doesn't exist.
func (p *AzureProvider) GetOrCreateResourceGroup(
	ctx context.Context,
) (*armresources.ResourceGroup, error) {
	resourceGroupName := p.Config.GetString("azure.resource_group_name")
	location := p.Config.GetString("azure.location")

	resourceGroup, err := p.Client.GetResourceGroup(ctx, location, resourceGroupName)
	if err != nil {
		if azureErr, ok := err.(AzureError); ok && azureErr.IsNotFound() {
			// Resource group not found; attempt to create it.
			resourceGroup, err = p.Client.GetOrCreateResourceGroup(
				ctx,
				location,
				resourceGroupName,
				nil,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create resource group: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get resource group: %w", err)
		}
	}

	return resourceGroup, nil
}

// DestroyResourceGroup deletes the specified resource group.
func (p *AzureProvider) DestroyResourceGroup(ctx context.Context) error {
	resourceGroupName := p.Config.GetString("azure.resource_group_name")
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

// PollAndUpdateResources polls Azure resources and updates their states.
func (p *AzureProvider) PollAndUpdateResources(ctx context.Context) ([]interface{}, error) {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return nil, fmt.Errorf("global model or deployment is nil")
	}

	subscriptionID := m.Deployment.Azure.SubscriptionID
	tags := m.Deployment.Azure.Tags

	resources, err := p.Client.ListAllResourcesInSubscription(ctx, subscriptionID, tags)
	if err != nil {
		return nil, err
	}

	// Implement logic to update the state of resources based on polling results.
	// For example, updating machine states, handling provisioning statuses, etc.

	return resources, nil
}

// GetVMExternalIP retrieves the external IP of a VM instance.
func (p *AzureProvider) GetVMExternalIP(
	ctx context.Context,
	resourceGroupName, vmName string,
) (string, error) {
	return p.Client.GetVMExternalIP(ctx, resourceGroupName, vmName)
}

// startUpdateProcessor processes update actions from the updateQueue.
func (p *AzureProvider) startUpdateProcessor(ctx context.Context) {
	l := logger.Get()
	if p == nil {
		l.Debug("startUpdateProcessor: Provider is nil")
		return
	}
	p.updateProcessorDone = make(chan struct{})
	l.Debug("startUpdateProcessor: Started")
	defer close(p.updateProcessorDone)
	defer l.Debug("startUpdateProcessor: Finished")

	for {
		select {
		case <-ctx.Done():
			l.Debug("startUpdateProcessor: Context cancelled")
			return
		case update, ok := <-p.updateQueue:
			if !ok {
				l.Debug("startUpdateProcessor: Update queue closed")
				return
			}
			l.Debug(
				fmt.Sprintf(
					"startUpdateProcessor: Processing update for %s, %s",
					update.MachineName,
					update.UpdateData.ResourceType,
				),
			)
			p.processUpdate(update)
		}
	}
}

// processUpdate applies the update action to the specified machine.
func (p *AzureProvider) processUpdate(update common.UpdateAction) {
	l := logger.Get()
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil || m.Deployment.Machines == nil {
		l.Debug("processUpdate: Global model, deployment, or machines is nil")
		return
	}

	machine, ok := m.Deployment.Machines[update.MachineName]
	if !ok {
		l.Debug(fmt.Sprintf("processUpdate: Machine %s not found", update.MachineName))
		return
	}

	if update.UpdateFunc == nil {
		l.Error("processUpdate: UpdateFunc is nil")
		return
	}

	switch update.UpdateData.UpdateType {
	case common.UpdateTypeComplete:
		machine.SetComplete()
	case common.UpdateTypeResource:
		machine.SetMachineResourceState(
			update.UpdateData.ResourceType.ResourceString,
			update.UpdateData.ResourceState,
		)
	case common.UpdateTypeService:
		machine.SetServiceState(update.UpdateData.ServiceType.Name, update.UpdateData.ServiceState)
	default:
		l.Errorf("processUpdate: Unknown UpdateType %s", update.UpdateData.UpdateType)
	}

	update.UpdateFunc(machine, update.UpdateData)
}

// StartResourcePolling starts polling Azure resources for updates.
func (p *AzureProvider) StartResourcePolling(ctx context.Context) {
	l := logger.Get()
	writeToDebugLog("Starting StartResourcePolling")

	if os.Getenv("ANDAIME_TEST_MODE") == "true" {
		l.Debug("ANDAIME_TEST_MODE is set to true, skipping resource polling")
		return
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
				return
			}
			pollCount++
			start := time.Now()
			writeToDebugLog(fmt.Sprintf("Starting poll #%d", pollCount))

			resources, err := p.PollAndUpdateResources(ctx)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
				writeToDebugLog(fmt.Sprintf("Failed to poll and update resources: %v", err))
				p.CancelAllDeployments(ctx)
				return
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
			writeToDebugLog(fmt.Sprintf("PollAndUpdateResources #%d took %v", pollCount, elapsed))

			p.logDeploymentStatus()

			if allResourcesProvisioned && p.AllMachinesComplete() {
				writeToDebugLog(
					"All resources provisioned and machines completed, stopping resource polling",
				)
				return
			}

		case <-quit:
			l.Debug("Quit signal received, exiting resource polling")
			writeToDebugLog("Quit signal received, exiting resource polling")
			return
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
