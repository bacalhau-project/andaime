//nolint:sigchanyzer
package azure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

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

var (
	DefaultAllowedPorts = []int{22, 80, 443}
	deployment          *models.Deployment
)

var createAzureDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in Azure",
	Long:  `Create a deployment in Azure using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func GetAzureCreateDeploymentCmd() *cobra.Command {
	return createAzureDeploymentCmd
}

func printFinalState(disp *display.Display) {
	fmt.Println("Final Deployment State:")
	fmt.Println(disp.GetTableString())
	fmt.Println("\nLogged Buffer:")
	fmt.Println(logger.GlobalLoggedBuffer.String())
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()

	l.Info("Starting executeCreateDeployment")

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Create a channel to signal when cleanup is done
	cleanupDone := utils.CreateBoolChannel("azure_createDeployment_cleanupDone", 1)
	l.Debugf("Channel created: azure_cleanup_done")

	// Create a channel for error communication
	errorChan := utils.CreateErrorChannel("azure_createDeployment_errorChan", 1)
	l.Debugf("Channel created: azure_error_channel")

	// Create a channel to signal deployment completion
	deploymentDone := utils.CreateBoolChannel("azure_createDeployment_deploymentDone", 1)
	l.Debugf("Channel created: azure_deployment_done")

	// Catch panics and log them
	defer func() {
		if r := recover(); r != nil {
			l.Error(fmt.Sprintf("Panic recovered in executeCreateDeployment: %v", r))
			l.Error(string(debug.Stack()))
			l.Debugf("Closing channel: azure_error_channel")
			errorChan <- fmt.Errorf("panic occurred: %v", r)
		}
		cancel() // Cancel the context
		l.Debugf("Closing channel: azure_cleanup_done")
		utils.CloseChannel(cleanupDone)
		l.Info("Cleanup completed")

		// Debug information about open channels
		l.Debug("Checking for open channels:")
		utils.DebugOpenChannels()
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
	sigChan := utils.CreateSignalChannel("azure_createDeployment_signalChan", 1)
	l.Debugf("Channel created: azure_signal_channel")

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
			disp.Start()
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

	// Create ticker channel
	ticker := time.NewTicker(MillisecondsBetweenUpdates * time.Millisecond)
	defer ticker.Stop()

	tickerDone := make(chan struct{})
	defer close(tickerDone)

	go func() {
		defer l.Debug("Ticker goroutine exited")
		for {
			select {
			case <-ticker.C:
				allMachinesComplete := true
				for _, machine := range deployment.Machines {
					if machine.Status != models.MachineStatusComplete {
						allMachinesComplete = false
					}
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
					if allMachinesComplete {
						tickerDone <- struct{}{}
					}
				}
			case <-tickerDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start resource deployment in a goroutine
	go func() {
		err := azureProvider.DeployResources(ctx, deployment, disp)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to deploy resources: %s", err.Error())
			l.Error(errMsg)
			// Send the error to the error channel
			if errorChan != nil {
				errorChan <- fmt.Errorf(errMsg)
			}
		} else {
			l.Info("Azure deployment created successfully")
			cmd.Println("Azure deployment created successfully")
			utils.CloseChannel(deploymentDone)
		}
	}()

	// Wait for signal, error, or deployment completion
	select {
	case <-sigChan:
		l.Info("Interrupt signal received, initiating graceful shutdown")
		l.Debugf("Closing channel: azure_signal_channel")
		utils.CloseChannel(sigChan)
		printFinalState(disp)
		return nil
	case err := <-errorChan:
		l.Errorf("Error occurred during deployment: %v", err)
		l.Debugf("Closing channel: azure_error_channel")
		utils.CloseChannel(errorChan)
		printFinalState(disp)
		return err
	case <-deploymentDone:
		l.Info("Deployment completed successfully")
		printFinalState(disp)
	case <-ctx.Done():
		l.Info("Context cancelled, initiating graceful shutdown")
		printFinalState(disp)
		return ctx.Err()
	}

	// Enable pprof profiling
	_, _ = fmt.Fprintf(&logger.GlobalLoggedBuffer, "pprof at end of executeCreateDeployment\n")
	_ = pprof.Lookup("goroutine").WriteTo(&logger.GlobalLoggedBuffer, 1)

	// Close all channels and finalize
	utils.CloseAllChannels()
	l.Debug("All channels closed - at the end of executeCreateDeployment")
	disp.Stop()        // Ensure display is stopped
	disp.WaitForStop() // Wait for the display to fully stop
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
			Parameters: models.Parameters{
				Orchestrator: true,
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
	deployment.SubscriptionID = getSubscriptionID()

	// Extract SSH keys
	sshPublicKeyPath, sshPrivateKeyPath, sshPublicKeyData, err := ExtractSSHKeyPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to extract SSH keys: %w", err)
	}

	// Ensure tags
	tags := utils.EnsureAzureTags(make(map[string]*string), projectID, uniqueID)

	// Validate SSH keys
	if err := sshutils.ValidateSSHKeysFromPath(sshPublicKeyPath, sshPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %w", err)
	}

	// Unmarshal machines configuration
	var rawMachines []models.Machine
	if err := viper.UnmarshalKey("azure.machines", &rawMachines); err != nil {
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
	deployment.SSHPublicKeyMaterial = sshPublicKeyData

	// Set ResourceGroupName only if it's not already set
	if deployment.ResourceGroupName == "" {
		resourceGroupName := viper.GetString("azure.resource_group_name")
		if resourceGroupName == "" {
			return nil, fmt.Errorf("resource group name is empty")
		}
		deployment.ResourceGroupName = resourceGroupName
	}

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
		rawMachines,
		disp,
		deployment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	deployment.OrchestratorNode = orchestratorNode
	deployment.Machines = allMachines
	deployment.Locations = locations

	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:        machine.ID,
			Status:    "Initializing machine...",
			Location:  machine.Location,
			StartTime: time.Now(),
		})
	}
	return deployment, nil
}

// ExtractSSHKeyPaths retrieves the paths for SSH public and private keys and the content of the public key.
//
// This function extracts the file paths for the SSH public and private keys from the configuration,
// reads the content of the public key file, and returns the necessary information for SSH authentication.
//
// Returns:
//   - string: The file path of the SSH public key.
//   - string: The file path of the SSH private key.
//   - string: The content of the SSH public key file, trimmed of newline characters.
//   - error: An error if any step in the process fails, such as extracting key paths or reading the public key file.
func ExtractSSHKeyPaths() (string, string, string, error) {
	publicKeyPath, err := extractSSHKeyPath("general.ssh_public_key_path")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to extract public key material: %w", err)
	}

	privateKeyPath, err := extractSSHKeyPath("general.ssh_private_key_path")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to extract private key material: %w", err)
	}

	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read public key file: %w", err)
	}

	returnPublicKeyData := strings.TrimSpace(string(publicKeyData))

	return publicKeyPath, privateKeyPath, returnPublicKeyData, nil
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
	rawMachines []models.Machine,
	disp *display.Display,
	deployment *models.Deployment,
) (*models.Machine, []models.Machine, []string, error) {
	var orchestratorNode *models.Machine
	var allMachines []models.Machine
	locations := make(map[string]bool)

	machines := make([]models.Machine, 0)
	for _, rawMachine := range rawMachines {
		var thisMachine models.Machine
		thisMachine.DiskSizeGB = deployment.DefaultDiskSizeGB
		thisMachine.VMSize = deployment.DefaultVMSize

		// Upsert machine parameters from rawMachine
		if (rawMachine.Parameters != models.Parameters{}) {
			if rawMachine.Parameters.Type != "" {
				thisMachine.VMSize = rawMachine.Parameters.Type
			}
		}

		if rawMachine.Parameters.Orchestrator {
			thisMachine.Parameters.Orchestrator = rawMachine.Parameters.Orchestrator
		}

		// If rawMachine.Parameters.Count is an int, and > 1, then replicate this machine
		if rawMachine.Parameters.Count > 1 {
			for i := 0; i < rawMachine.Parameters.Count; i++ {
				machines = append(machines, thisMachine)
			}
		} else {
			machines = append(machines, thisMachine)
		}
	}

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

		if (machine.Parameters != models.Parameters{}) && (machine.Parameters.Orchestrator) {
			if orchestratorNode != nil {
				return nil, nil, nil, fmt.Errorf("multiple orchestrator nodes found")
			}
			orchestratorNode = &internalMachine
		}
		allMachines = append(allMachines, internalMachine)
	}

	uniqueLocations := make([]string, 0, len(locations))
	for location := range locations {
		uniqueLocations = append(uniqueLocations, location)
	}

	return orchestratorNode, allMachines, uniqueLocations, nil
}
