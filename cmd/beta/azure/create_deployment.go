//nolint:sigchanyzer
package azure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()
	prog := display.GetGlobalProgram()

	logger.SetLevel(logger.DEBUG)
	l.Info("Starting executeCreateDeployment")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	UniqueID := time.Now().Format("060102150405")
	l.Infof("Generated UniqueID: %s", UniqueID)
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, UniqueID)

	l.Debug("Initializing Azure provider")
	p, err := azure.AzureProviderFunc()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to initialize Azure provider: %s", err.Error())
		l.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	l.Info("Azure provider initialized successfully")

	deployment, err := InitializeDeployment(ctx, UniqueID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to initialize deployment: %s", err.Error())
		l.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	l.Info("Starting resource deployment")

	// Initialize and run bubbletea program
	m := display.InitialModel()
	m.Deployment = deployment
	prog.InitProgram(m)

	go func() {
		// Start Resource Polling
		go p.StartResourcePolling(ctx)
	}()

	var deploymentErr error
	go func() {
		if err := p.DeployResources(ctx); err != nil {
			deploymentErr = fmt.Errorf("failed to deploy resources: %s", err.Error())
			l.Error(deploymentErr.Error())
			prog.Quit()
			return
		}
		l.Info("Azure deployment created successfully")
		if err := p.FinalizeDeployment(context.Background()); err != nil {
			deploymentErr = fmt.Errorf("failed to finalize deployment: %v", err)
			l.Error(deploymentErr.Error())
			prog.Quit()
			return
		}
		l.Info("Deployment finalized")
		time.Sleep(2 * time.Second) // Wait for 2 seconds before quitting
		prog.Quit()
	}()

	_, err = prog.Run()
	if err != nil {
		l.Error(fmt.Sprintf("Error running program: %v", err))
		return err
	}

	if deploymentErr != nil {
		l.Error(deploymentErr.Error())
		return deploymentErr
	}

	return nil
}

// initializeDeployment prepares the deployment configuration
func InitializeDeployment(
	ctx context.Context,
	uniqueID string,
) (*models.Deployment, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("deployment cancelled before starting: %w", err)
	}

	// Set default values for all configuration items
	viper.SetDefault("general.project_id", "default-project")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", "info")
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("azure.resource_group_name", "andaime-rg")
	viper.SetDefault("azure.resource_group_location", "eastus")
	viper.SetDefault("azure.allowed_ports", globals.DefaultAllowedPorts)
	viper.SetDefault("azure.default_vm_size", "Standard_B2s")
	viper.SetDefault("azure.default_disk_size_gb", globals.DefaultDiskSizeGB)
	viper.SetDefault("azure.default_location", "eastus")
	viper.SetDefault("azure.machines", []models.Machine{
		{
			Name:     "default-vm",
			VMSize:   viper.GetString("azure.default_vm_size"),
			Location: viper.GetString("azure.default_location"),
			Parameters: models.Parameters{
				Orchestrator: true,
			},
		},
	})

	// Extract Azure-specific configuration
	projectID := viper.GetString("general.project_id")

	// Create deployment object
	deployment, err := PrepareDeployment(ctx, projectID, uniqueID)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	return deployment, nil
}

// prepareDeployment sets up the initial deployment configuration
func PrepareDeployment(
	ctx context.Context,
	projectID, uniqueID string,
) (*models.Deployment, error) {
	l := logger.Get()

	deployment := &models.Deployment{}
	deployment.ResourceGroupName = viper.GetString("azure.resource_group_name")
	deployment.ResourceGroupLocation = viper.GetString("azure.resource_group_location")
	deployment.AllowedPorts = viper.GetIntSlice("azure.allowed_ports")
	deployment.DefaultVMSize = viper.GetString("azure.default_vm_size")
	deployment.DefaultDiskSizeGB = int32(viper.GetInt("azure.default_disk_size_gb"))
	deployment.DefaultLocation = viper.GetString("azure.default_location")
	deployment.SubscriptionID = getSubscriptionID()

	// Extract SSH keys
	var err error
	deployment.SSHPublicKeyPath,
		deployment.SSHPrivateKeyPath,
		deployment.SSHPublicKeyMaterial,
		deployment.SSHPrivateKeyMaterial,
		err = ExtractSSHKeyPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to extract SSH keys: %w", err)
	}

	// Ensure tags
	tags := utils.EnsureAzureTags(make(map[string]*string), projectID, uniqueID)

	// Validate SSH keys - do this early so we can fail fast
	if err := sshutils.ValidateSSHKeysFromPath(deployment.SSHPublicKeyPath, deployment.SSHPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %w", err)
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
		deployment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	deployment.OrchestratorNode = orchestratorNode
	deployment.Machines = allMachines
	deployment.Locations = locations

	for _, machine := range deployment.Machines {
		_ = machine // TODO: Figure out how to pass around model
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
//   - string: The content of the SSH private key file, trimmed of newline characters.
//   - error: An error if any step in the process fails, such as extracting key paths or reading the public key file.
func ExtractSSHKeyPaths() (string, string, string, string, error) {
	publicKeyPath, err := extractSSHKeyPath("general.ssh_public_key_path")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to extract public key material: %w", err)
	}

	privateKeyPath, err := extractSSHKeyPath("general.ssh_private_key_path")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to extract private key material: %w", err)
	}

	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read public key file: %w", err)
	}

	returnPublicKeyData := strings.TrimSpace(string(publicKeyData))

	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read private key file: %w", err)
	}
	returnPrivateKeyData := strings.TrimSpace(string(privateKeyData))

	return publicKeyPath,
		privateKeyPath,
		returnPublicKeyData,
		returnPrivateKeyData,
		nil
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
	deployment *models.Deployment,
) (*models.Machine, []models.Machine, []string, error) {
	var orchestratorNode *models.Machine
	var allMachines []models.Machine
	locations := make(map[string]bool)

	type MachineConfig struct {
		Location   string `yaml:"location"`
		Parameters struct {
			Count        int    `yaml:"count,omitempty"`
			Type         string `yaml:"type,omitempty"`
			Orchestrator bool   `yaml:"orchestrator,omitempty"`
		} `yaml:"parameters"`
	}
	var rawMachines []MachineConfig
	if err := viper.UnmarshalKey("azure.machines", &rawMachines); err != nil {
		return nil, nil, nil, fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt("azure.default_count_per_zone")
	defaultType := viper.GetString("azure.default_machine_type")
	defaultDiskSize := viper.GetInt("azure.disk_size_gb")

	for _, rawMachine := range rawMachines {
		var thisMachine models.Machine
		thisMachine.Type = models.AzureResourceTypeVM
		thisMachine.DiskSizeGB = int32(defaultDiskSize)
		thisMachine.VMSize = defaultType
		thisMachine.Location = rawMachine.Location
		thisMachine.Parameters.Orchestrator = false

		// Upsert machine parameters from rawMachine
		if rawMachine.Parameters.Type != "" {
			thisMachine.VMSize = rawMachine.Parameters.Type
		}

		if rawMachine.Parameters.Orchestrator {
			if orchestratorNode != nil || rawMachine.Parameters.Count > 1 {
				return nil, nil, nil, fmt.Errorf("multiple orchestrator nodes found")
			}
			thisMachine.Orchestrator = rawMachine.Parameters.Orchestrator
			orchestratorNode = &thisMachine
		}

		countOfMachines := rawMachine.Parameters.Count
		if countOfMachines == 0 {
			countOfMachines = defaultCount
		}
		for i := 0; i < countOfMachines; i++ {
			thisMachine.ID = utils.CreateShortID()
			thisMachine.Name = fmt.Sprintf("%s-vm", thisMachine.ID)
			thisMachine.ComputerName = fmt.Sprintf("%s-vm", thisMachine.ID)
			thisMachine.StartTime = time.Now()

			thisMachine.SSH = models.ServiceStateNotStarted
			thisMachine.Docker = models.ServiceStateNotStarted
			thisMachine.Bacalhau = models.ServiceStateNotStarted

			allMachines = append(allMachines, thisMachine)
		}
	}

	uniqueLocations := make([]string, 0, len(locations))
	for location := range locations {
		uniqueLocations = append(uniqueLocations, location)
	}

	return orchestratorNode, allMachines, uniqueLocations, nil
}
