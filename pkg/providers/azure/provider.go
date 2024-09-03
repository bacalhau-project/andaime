package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/providers/general"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/blang/semver"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// AzureClienter interface defines the methods we need for Azure operations
type AzureClienter interface {
	ResourceGroupExists(ctx context.Context, resourceGroupName string) (bool, error)
	DestroyResourceGroup(ctx context.Context, resourceGroupName string) error
	ListAllResourcesInSubscription(ctx context.Context, subscriptionID string, tags map[string]*string) ([]interface{}, error)
	// Add other methods as needed
}

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
type AzureProviderer interface {
	general.Providerer
	common.ClusterDeployer
	GetAzureClient() AzureClienter
	SetAzureClient(client AzureClienter)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)

	StartResourcePolling(ctx context.Context)
	PrepareResourceGroup(ctx context.Context) error
	DeployResources(ctx context.Context) error
	ProvisionPackagesOnMachines(ctx context.Context) error
	ProvisionBacalhau(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	DestroyResources(ctx context.Context, resourceGroupName string) error
	PollAndUpdateResources(ctx context.Context) ([]interface{}, error)
}

type AzureProvider struct {
	common.BaseClusterDeployer
	Client              interface{}
	Config              *viper.Viper
	SSHClient           sshutils.SSHClienter
	SSHUser             string
	SSHPort             int
	lastResourceQuery   time.Time
	cachedResources     []interface{}
	updateQueue         chan UpdateAction
	updateMutex         sync.Mutex
	updateProcessorDone chan struct{}
	serviceMutex        sync.Mutex //nolint:unused
	servicesProvisioned bool       //nolint:unused
}

var NewAzureProviderFunc = NewAzureProvider

// NewAzureProvider creates a new AzureProvider instance
func NewAzureProvider() (AzureProviderer, error) {
	config := viper.GetViper()
	if !config.IsSet("azure") {
		return nil, fmt.Errorf("azure configuration is required")
	}

	if !config.IsSet("azure.subscription_id") {
		return nil, fmt.Errorf("azure.subscription_id is required")
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

	subscriptionID := config.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure.subscription_id is empty or not set in the configuration")
	}
	l := logger.Get()
	l.Debugf("Using Azure subscription ID: %s", subscriptionID)

	// Validate the subscription ID format
	if !isValidGUID(subscriptionID) {
		return nil, fmt.Errorf("invalid Azure subscription ID format: %s", subscriptionID)
	}

	client, err := NewAzureClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	sshUser := config.GetString("general.ssh_user")
	if sshUser == "" {
		sshUser = "azureuser" // Default SSH user for Azure VMs
	}

	sshPort := config.GetInt("general.ssh_port")
	if sshPort == 0 {
		sshPort = 22 // Default SSH port
	}

	provider := &AzureProvider{
		Client:      client,
		Config:      config,
		SSHUser:     sshUser,
		SSHPort:     sshPort,
		updateQueue: make(chan UpdateAction, UpdateQueueSize),
	}

	go provider.startUpdateProcessor(context.Background())

	// Initialize the display model with machines from the configuration
	provider.initializeDisplayModel()

	return provider, nil
}

func (p *AzureProvider) initializeDisplayModel() {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		l := logger.Get()
		l.Error("Global model or deployment is nil")
		return
	}
}

func (p *AzureProvider) GetAzureClient() AzureClienter {
	if client, ok := p.Client.(AzureClienter); ok {
		return client
	}
	return nil
}

func (p *AzureProvider) SetAzureClient(client AzureClienter) {
	p.Client = client
}

func (p *AzureProvider) GetSSHClient() sshutils.SSHClienter {
	return p.SSHClient
}

func (p *AzureProvider) SetSSHClient(client sshutils.SSHClienter) {
	p.SSHClient = client
}

func (p *AzureProvider) GetConfig() *viper.Viper {
	return p.Config
}

func (p *AzureProvider) SetConfig(config *viper.Viper) {
	p.Config = config
}

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

func (p *AzureProvider) processUpdate(update UpdateAction) {
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

	if update.UpdateData.UpdateType == UpdateTypeComplete {
		machine.SetComplete()
	} else if update.UpdateData.UpdateType == UpdateTypeResource {
		machine.SetResourceState(update.UpdateData.ResourceType.ResourceString, update.UpdateData.ResourceState)
	} else if update.UpdateData.UpdateType == UpdateTypeService {
		machine.SetServiceState(update.UpdateData.ServiceType.Name, update.UpdateData.ServiceState)
	}

	update.UpdateFunc(machine, update.UpdateData)
}

func (p *AzureProvider) DestroyResources(ctx context.Context, resourceGroupName string) error {
	l := logger.Get()
	client := p.GetAzureClient()

	// Check if the resource group exists before attempting to destroy it
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

func (p *AzureProvider) CreateResources(ctx context.Context) error {
	// TODO: Implement resource creation
	return nil
}

func (p *AzureProvider) ProvisionSSH(ctx context.Context) error {
	// TODO: Implement SSH provisioning
	return nil
}

func (p *AzureProvider) SetupDocker(ctx context.Context) error {
	// TODO: Implement Docker setup
	return nil
}

func (p *AzureProvider) DeployOrchestrator(ctx context.Context) error {
	// TODO: Implement orchestrator deployment
	return nil
}

func (p *AzureProvider) DeployNodes(ctx context.Context) error {
	// TODO: Implement node deployment
	return nil
}

// Updates the deployment with the latest resource state
func (p *AzureProvider) ListAllResourcesInSubscription(ctx context.Context,
	subscriptionID string,
	tags map[string]*string) ([]interface{}, error) {
	l := logger.Get()
	l.Debugf("ListAllResourcesInSubscription called with subscriptionID: %s", subscriptionID)
	client := p.GetAzureClient()

	// Check if we can use cached results
	if time.Since(p.lastResourceQuery) < 30*time.Second {
		l.Debug("Using cached resources")
		return p.cachedResources, nil
	}

	start := time.Now()
	resources, err := client.ListAllResourcesInSubscription(ctx,
		subscriptionID,
		tags)
	elapsed := time.Since(start)
	l.Debugf("ListAllResourcesInSubscription took %v", elapsed)
	writeToDebugLog(fmt.Sprintf("ListAllResourcesInSubscription took %v", elapsed))

	if err != nil {
		l.Errorf("Failed to query Azure resources: %v", err)
		writeToDebugLog(fmt.Sprintf("Failed to query Azure resources: %v", err))
		return nil, fmt.Errorf("failed to query resources: %v", err)
	}

	if resources == nil {
		resources = []interface{}{}
	}

	// Update cache
	p.cachedResources = resources
	p.lastResourceQuery = time.Now()

	return resources, nil
}

func (p *AzureProvider) StartResourcePolling(ctx context.Context) {
	l := logger.Get()
	writeToDebugLog("Starting StartResourcePolling")

	resourceTicker := time.NewTicker(ResourcePollingInterval)

	quit := make(chan struct{})
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

			writeToDebugLog(
				fmt.Sprintf("Poll #%d: Found %d resources", pollCount, len(resources)),
			)

			allResourcesProvisioned := true
			for _, resource := range resources {
				resourceMap := resource.(map[string]interface{})
				provisioningState := resourceMap["provisioningState"].(string)
				writeToDebugLog(
					fmt.Sprintf(
						"Resource: %s - Provisioning State: %s",
						resourceMap["name"].(string),
						provisioningState,
					),
				)
				if provisioningState != "Succeeded" {
					allResourcesProvisioned = false
				}
			}

			elapsed := time.Since(start)
			writeToDebugLog(
				fmt.Sprintf("PollAndUpdateResources #%d took %v", pollCount, elapsed),
			)

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
			resourceTicker.Stop()
			return
		}
	}
}

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
			m.Deployment.ResourceGroupName,
		),
	)
	writeToDebugLog(fmt.Sprintf("Total Machines: %d", len(m.Deployment.Machines)))

	for _, machine := range m.Deployment.Machines {
		writeToDebugLog(
			fmt.Sprintf(
				"Machine Name: %s, PublicIP: %s, PrivateIP: %s",
				machine.Name,
				machine.PublicIP,
				machine.PrivateIP,
			),
		)
		writeToDebugLog(
			fmt.Sprintf(
				"Machine %s - Docker: %v, CorePackages: %v, Bacalhau: %v, SSH: %v",
				machine.Name,
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
				machine.Name,
				completedResources,
				totalResources,
			),
		)

		if machine.GetMachineResources() != nil {
			for resourceType, resource := range machine.GetMachineResources() {
				writeToDebugLog(
					fmt.Sprintf(
						"Machine %s - Resource %s: State: %v, Value: %s",
						machine.Name,
						resourceType,
						resource.ResourceState,
						resource.ResourceValue,
					),
				)
			}
		} else {
			writeToDebugLog(fmt.Sprintf("Machine %s - No machine resources", machine.Name))
		}
	}
}

var _ AzureProviderer = &AzureProvider{}

func writeToDebugLog(message string) {
	debugFilePath := "/tmp/andaime-debug.log"
	debugFile, err := os.OpenFile(
		debugFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644, //nolint:mnd
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

func (p *AzureProvider) CancelAllDeployments(ctx context.Context) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	l.Info("Cancelling all deployments")
	writeToDebugLog("Cancelling all deployments")

	for _, machine := range m.Deployment.Machines {
		if !machine.Complete() {
			machine.SetComplete()
		}
	}

	// Cancel the context to stop all ongoing operations
	if cancelFunc, ok := ctx.Value("cancelFunc").(context.CancelFunc); ok {
		cancelFunc()
	}
}

func (p *AzureProvider) AllMachinesComplete() bool {
	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		if !machine.Complete() {
			return false
		}
	}
	return true
}

func stripAndParseJSON(input string) ([]map[string]interface{}, error) {
	// Find the start of the JSON array
	start := strings.Index(input, "[")
	if start == -1 {
		return nil, fmt.Errorf("no JSON array found in input")
	}

	// Extract the JSON part
	jsonStr := input[start:]

	// Parse the JSON
	var result []map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return result, nil
}

func isValidGUID(guid string) bool {
	r := regexp.MustCompile(
		"^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$",
	)
	return r.MatchString(guid)
}

func (p *AzureProvider) WaitForAllMachinesToReachState(
	ctx context.Context,
	targetState models.AzureResourceState,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	for {
		allReady := true
		for _, machine := range m.Deployment.Machines {
			state := machine.GetResourceState("Microsoft.Compute/virtualMachines")
			if err := p.TestSSHLiveness(ctx, machine.Name); err != nil {
				return err
			}

			l.Debugf("Machine %s state: %d", machine.Name, state)
			if state != targetState {
				allReady = false
				break
			}
		}
		if allReady {
			l.Debug("All machines have reached the target state")
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(WaitingForMachinesTime):
			l.Debug("Waiting for machines to reach target state...")
		}
	}
	return nil
}

// TestSSHLiveness tests the SSH liveness of a machine
func (p *AzureProvider) TestSSHLiveness(ctx context.Context, machineName string) error {
	// Implement the SSH liveness test
	return nil
}

// PollAndUpdateResources polls and updates Azure resources
func (p *AzureProvider) PollAndUpdateResources(ctx context.Context) ([]interface{}, error) {
	// Implement resource polling and updating
	return nil, nil
}

// NewAzureClient creates a new Azure client
func NewAzureClient(subscriptionID string) (AzureClienter, error) {
	// Implement Azure client creation
	return nil, nil
}

func verifyDocker(ctx context.Context, mach *models.Machine) error {
	m := display.GetGlobalModelFunc()

	sshConfig, err := sshutils.NewSSHConfigFunc(
		mach.PublicIP,
		mach.SSHPort,
		mach.SSHUser,
		mach.SSHPrivateKeyMaterial,
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	m.UpdateStatus(
		models.NewDisplayStatusWithText(
			mach.Name,
			models.AzureResourceTypeVM,
			models.AzureResourceStatePending,
			"Testing Docker",
		),
	)

	versionsObject := map[string]interface{}{}
	out, err := sshConfig.ExecuteCommand(ctx, "sudo docker version -f json")
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	err = json.Unmarshal([]byte(out), &versionsObject)
	if err != nil {
		return fmt.Errorf("failed to marshal Docker server version: %w", err)
	}

	serverVersionDetected := false
	clientVersionDetected := false
	if versionsObject["Server"] == nil {
		return fmt.Errorf("failed to get Docker server version")
	} else {
		if serverVersion, ok := versionsObject["Server"].(map[string]interface{}); ok {
			if version, ok := serverVersion["Version"].(string); ok {
				// If Version is a semver, we can use it
				if _, err := semver.Parse(version); err == nil {
					serverVersionDetected = true
				} else {
					serverVersionDetected = false
				}
			}
		}
	}

	if versionsObject["Client"] == nil {
		return fmt.Errorf("failed to get Docker client version")
	} else {
		if clientVersion, ok := versionsObject["Client"].(map[string]interface{}); ok {
			if version, ok := clientVersion["Version"].(string); ok {
				// If Version is a semver, we can use it
				if _, err := semver.Parse(version); err == nil {
					clientVersionDetected = true
				}
			}
		}
	}

	if !serverVersionDetected || !clientVersionDetected {
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				mach.Name,
				models.AzureResourceTypeVM,
				models.AzureResourceStateFailed,
				"Failed to detect Docker version",
			),
		)
		return fmt.Errorf("failed to detect Docker version")
	}

	// If all checks pass, continue with the existing code
	mach.StatusMessage = "Successfully Deployed"
	mach.SetServiceState("Docker", models.ServiceStateSucceeded)
	m.UpdateStatus(
		models.NewDisplayStatusWithText(
			mach.Name,
			models.AzureResourceTypeVM,
			models.AzureResourceStateSucceeded,
			"Docker Successfully Deployed",
		),
	)

	return nil
}
