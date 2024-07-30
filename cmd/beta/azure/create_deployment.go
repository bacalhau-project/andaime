//nolint:sigchanyzer
package azure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type contextKey string

const uniqueDeploymentIDKey contextKey = "UniqueDeploymentID"
const MillisecondsBetweenUpdates = 100
const DefaultDiskSizeGB = 30

var DefaultAllowedPorts = []int{22, 80, 443}

var createAzureDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in Azure",
	Long:  `Create a deployment in Azure using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func GetAzureCreateDeploymentCmd() *cobra.Command {
	return createAzureDeploymentCmd
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	logger.InitProduction(false, true)
	l := logger.Get()
	l.Info("Starting executeCreateDeployment")

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Create a channel to signal when cleanup is done
	cleanupDone := utils.CreateStructChannel(1)

	// Create a channel for error communication
	errorChan := utils.CreateErrorChannel(1)

	// Catch panics and log them
	defer func() {
		if r := recover(); r != nil {
			l.Error(fmt.Sprintf("Panic recovered in executeCreateDeployment: %v", r))
			l.Error(string(debug.Stack()))
			errorChan <- fmt.Errorf("panic occurred: %v", r)
		}
		cancel() // Cancel the context
		utils.CloseChannel(cleanupDone)
		l.Info("Cleanup completed")
	}()

	// Create a unique ID for the deployment
	UniqueID := time.Now().Format("060102150405")
	l.Infof("Generated UniqueID: %s", UniqueID)

	// Set the UniqueID on the context
	ctx = context.WithValue(ctx, uniqueDeploymentIDKey, UniqueID)

	l.Debug("Initializing Azure provider")
	azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
	if err != nil {
		errMsg := fmt.Sprintf("Failed to initialize Azure provider: %s", err.Error())
		l.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	l.Info("Azure provider initialized successfully")

	l.Debug("Setting up signal channel")
	sigChan := utils.CreateSignalChannel(1)

	signal.Notify(
		sigChan,
		os.Interrupt,
		syscall.SIGTERM,
	)

	l.Debug("Creating display")
	disp := display.GetGlobalDisplay()
	l.Debug("Starting display")

	noDisplay := false
	if os.Getenv("ANDAIME_NO_DISPLAY") != "" {
		noDisplay = true
	}
	if !noDisplay {
		// Start display in a goroutine
		go func() {
			l.Debug("Display Start() called")
			summaryReceived := utils.CreateStructChannel(1)
			disp.Start(sigChan, summaryReceived)
			l.Debug("Display Start() returned")
		}()

		// Ensure display is stopped in all scenarios
		defer func() {
			l.Debug("Stopping display")
			disp.Stop()
			l.Debug("Display stopped")
			utils.CloseAllChannels()
		}()
	}

	// Create a new deployment object
	deployment, err := InitializeDeployment(ctx, UniqueID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to initialize deployment: %s", err.Error())
		l.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	l.Info("Starting resource deployment")

	for _, machine := range deployment.Machines {
		l.Debugf("Deploying machine: %s", machine.Name)
		disp.UpdateStatus(&models.Status{
			ID:        machine.ID,
			Status:    "Initializing",
			Type:      "VM",
			StartTime: time.Now(),
		})
	}

	// Create ticker channel
	ticker := time.NewTicker(MillisecondsBetweenUpdates * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			for _, machine := range deployment.Machines {
				if machine.Status == models.MachineStatusComplete {
					continue
				}
				disp.UpdateStatus(&models.Status{
					ID: machine.ID,
					ElapsedTime: time.Duration(
						time.Since(machine.StartTime).
							Milliseconds() /
							1000, //nolint:gomnd // Divide by 1000 to convert milliseconds to seconds
					),
				})
			}
		}
	}()

	// Start resource deployment in a goroutine
	go func() {
		err := azureProvider.DeployResources(ctx, deployment, disp)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to deploy resources: %s", err.Error())
			l.Error(errMsg)
			errorChan <- fmt.Errorf(errMsg)
		} else {
			l.Info("Azure deployment created successfully")
			cmd.Println("Azure deployment created successfully")
			cmd.Println("Press 'q' and Enter to quit")
		}
	}()

	// Wait for signal, error, or user input
	select {
	case <-sigChan:
		l.Info("Interrupt signal received, initiating graceful shutdown")
		disp.Close()
		l.Info("Display closed - sigChan")
	case err := <-errorChan:
		l.Errorf("Error occurred during deployment: %v", err)
		disp.Close()
		l.Info("Display closed - errorChan")
		return err
	case <-ctx.Done():
		l.Info("Context cancelled, initiating graceful shutdown")
		disp.Close()
		l.Info("Display closed - ctx.Done()")
	}

	utils.CloseAllChannels()

	return nil
}

// initializeDeployment prepares the deployment configuration
func InitializeDeployment(
	ctx context.Context,
	uniqueID string,
) (*models.Deployment, error) {
	v := viper.GetViper()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("deployment cancelled before starting: %w", err)
	}

	// Set default values for all configuration items
	v.SetDefault("general.project_id", "default-project")
	v.SetDefault("general.log_path", "/var/log/andaime")
	v.SetDefault("general.log_level", "info")
	v.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	v.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	v.SetDefault("azure.resource_group_name", "andaime-rg")
	v.SetDefault("azure.resource_group_location", "eastus")
	v.SetDefault("azure.allowed_ports", DefaultAllowedPorts)
	v.SetDefault("azure.default_vm_size", "Standard_B2s")
	v.SetDefault("azure.default_disk_size_gb", DefaultDiskSizeGB)
	v.SetDefault("azure.default_location", "eastus")
	v.SetDefault("azure.machines", []models.Machine{
		{
			Name:     "default-vm",
			VMSize:   v.GetString("azure.default_vm_size"),
			Location: v.GetString("azure.default_location"),
			Parameters: []models.Parameters{
				{Orchestrator: true},
			},
		},
	})

	// Extract Azure-specific configuration
	projectID := v.GetString("general.project_id")

	// Create deployment object
	deployment, err := PrepareDeployment(ctx, v, projectID, uniqueID)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	return deployment, nil
}

func EnsureTags(tags map[string]*string, projectID, uniqueID string) map[string]*string {
	if tags == nil {
		tags = map[string]*string{}
	}
	if tags["andaime"] == nil {
		tags["andaime"] = to.Ptr("true")
	}
	if tags["deployed-by"] == nil {
		tags["deployed-by"] = to.Ptr("andaime")
	}
	if tags["andaime-resource-tracking"] == nil {
		tags["andaime-resource-tracking"] = to.Ptr("true")
	}
	if tags["unique-id"] == nil {
		tags["unique-id"] = to.Ptr(uniqueID)
	}
	if tags["project-id"] == nil {
		tags["project-id"] = to.Ptr(projectID)
	}
	if tags["andaime-project"] == nil {
		tags["andaime-project"] = to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID))
	}
	return tags
}

// prepareDeployment sets up the initial deployment configuration
func PrepareDeployment(
	ctx context.Context,
	viper *viper.Viper,
	projectID, uniqueID string,
) (*models.Deployment, error) {
	l := logger.Get()
	disp := display.GetGlobalDisplay()

	deployment := &models.Deployment{}
	deployment.ResourceGroupName = viper.GetString("azure.resource_group_name")
	deployment.ResourceGroupLocation = viper.GetString("azure.resource_group_location")
	deployment.AllowedPorts = viper.GetIntSlice("azure.allowed_ports")
	deployment.DefaultVMSize = viper.GetString("azure.default_vm_size")
	deployment.DefaultDiskSizeGB = int32(viper.GetInt("azure.default_disk_size_gb"))
	deployment.DefaultLocation = viper.GetString("azure.default_location")

	// Extract SSH keys
	sshPublicKeyPath, sshPrivateKeyPath, sshPublicKeyData, err := ExtractSSHKeyPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to extract SSH keys: %w", err)
	}

	// Ensure tags
	tags := EnsureTags(make(map[string]*string), projectID, uniqueID)

	// Validate SSH keys
	if err := sshutils.ValidateSSHKeysFromPath(sshPublicKeyPath, sshPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %w", err)
	}

	// Unmarshal machines configuration
	var machines []models.Machine
	if err := viper.UnmarshalKey("azure.machines", &machines); err != nil {
		return nil, fmt.Errorf("error unmarshaling machines: %w", err)
	}

	// Get resource group location
	resourceGroupLocation := viper.GetString("azure.resource_group_location")
	if resourceGroupLocation == "" {
		return nil, fmt.Errorf("resource group location is empty")
	}

	deployment.ProjectID = projectID
	deployment.UniqueID = uniqueID
	deployment.ResourceGroupLocation = resourceGroupLocation
	deployment.Tags = tags
	deployment.SSHPublicKeyData = sshPublicKeyData

	// Set ResourceGroupName only if it's not already set
	if deployment.ResourceGroupName == "" {
		resourceGroupName := viper.GetString("azure.resource_group_name")
		if resourceGroupName == "" {
			return nil, fmt.Errorf("resource group name is empty")
		}
		deployment.ResourceGroupName = resourceGroupName
	}
	deployment.ResourceGroupName = fmt.Sprintf("%s-%s", deployment.ResourceGroupName, uniqueID)

	// Ensure the deployment has a name
	if deployment.Name == "" {
		deployment.Name = fmt.Sprintf("Azure Deployment - %s", uniqueID)
	}

	// Update Viper configuration
	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %w", err)
	}

	// Get allowed ports
	ports := viper.GetIntSlice("azure.allowed_ports")
	if len(ports) == 0 {
		return nil, fmt.Errorf("no allowed ports found in viper config")
	}
	l.Debugf("Allowed ports: %v", ports)

	// Process machines
	orchestratorNode, allMachines, locations, err := ProcessMachinesConfig(
		machines,
		disp,
		deployment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	deployment.OrchestratorNode = orchestratorNode
	deployment.Machines = allMachines
	deployment.Locations = locations

	return deployment, nil
}

// extractSSHKeys extracts SSH public and private key contents from Viper configuration
func ExtractSSHKeyPaths() (string, string, []byte, error) {
	publicKeyPath, err := extractSSHKeyPath("general.ssh_public_key_path")
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to extract public key material: %w", err)
	}

	privateKeyPath, err := extractSSHKeyPath("general.ssh_private_key_path")
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to extract private key material: %w", err)
	}

	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	return publicKeyPath, privateKeyPath, publicKeyData, nil
}

func extractSSHKeyPath(configKeyString string) (string, error) {
	l := logger.Get()

	l.Debugf("Extracting key path from %s", configKeyString)
	keyPath := viper.GetString(configKeyString)
	if keyPath == "" {
		return "", fmt.Errorf(
			"%s is empty. \nConfig key used: %s \nConfig file used: %s",
			configKeyString,
			configKeyString,
			viper.ConfigFileUsed(),
		)
	}

	l.Debugf("Extracting key material from %s", keyPath)
	if keyPath == "" {
		return "", fmt.Errorf("key path is empty")
	}

	l.Debugf("Getting absolute path for key file: %s", keyPath)

	// If path starts with "~" then replace with homedir
	if strings.HasPrefix(keyPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[1:])
	}

	absoluteKeyPath, err := filepath.Abs(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for key file: %w", err)
	}

	l.Debugf("Reading key file: %s", absoluteKeyPath)
	keyMaterial, err := os.ReadFile(absoluteKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	if len(keyMaterial) == 0 {
		return "", fmt.Errorf("key file is empty")
	}

	return absoluteKeyPath, nil
}

// processMachinesConfig processes the machine configurations
func ProcessMachinesConfig(
	machines []models.Machine,
	disp *display.Display,
	deployment *models.Deployment,
) (*models.Machine, []models.Machine, []string, error) {
	var orchestratorNode *models.Machine
	var allMachines []models.Machine
	locations := make(map[string]bool)

	for _, machine := range machines {
		internalMachine := machine

		if internalMachine.Location == "" {
			internalMachine.Location = deployment.DefaultLocation
		}

		if !internal.IsValidLocation(internalMachine.Location) {
			return nil, nil, nil, fmt.Errorf("invalid location: %s", internalMachine.Location)
		}
		locations[internalMachine.Location] = true

		if internalMachine.VMSize == "" {
			internalMachine.VMSize = deployment.DefaultVMSize
		}

		if internalMachine.DiskSizeGB == 0 {
			internalMachine.DiskSizeGB = deployment.DefaultDiskSizeGB
		}

		internalMachine.ID = utils.CreateShortID()
		internalMachine.Name = fmt.Sprintf("vm-%s", internalMachine.ID)
		internalMachine.ComputerName = fmt.Sprintf("vm-%s", internalMachine.ID)
		internalMachine.StartTime = time.Now()

		if len(machine.Parameters) > 0 && machine.Parameters[0].Orchestrator {
			if orchestratorNode != nil {
				return nil, nil, nil, fmt.Errorf("multiple orchestrator nodes found")
			}
			orchestratorNode = &internalMachine
		}
		allMachines = append(allMachines, internalMachine)

		disp.UpdateStatus(&models.Status{
			ID:        internalMachine.ID,
			Type:      "VM",
			Location:  internalMachine.Location,
			Status:    "Initializing",
			StartTime: time.Now(),
		})
	}

	uniqueLocations := make([]string, 0, len(locations))
	for location := range locations {
		uniqueLocations = append(uniqueLocations, location)
	}

	return orchestratorNode, allMachines, uniqueLocations, nil
}
