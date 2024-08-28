package azure

import (
	"context"
	"fmt"
	"io"
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
	"golang.org/x/crypto/ssh"
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
	l.Info("Starting executeCreateDeployment")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	setDefaultConfigurations()

	projectID := viper.GetString("general.project_id")
	if projectID == "" {
		return fmt.Errorf("project ID is empty")
	}

	uniqueID := time.Now().Format("060102150405")
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, uniqueID)

	p, err := azure.AzureProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	m := display.InitialModel()
	deployment, err := PrepareDeployment(ctx, projectID, uniqueID)
	if err != nil {
		return fmt.Errorf("failed to initialize deployment: %w", err)
	}
	m.Deployment = deployment

	prog := display.GetGlobalProgram()
	prog.InitProgram(m)

	go p.StartResourcePolling(ctx)

	var deploymentErr error
	go func() {
		select {
		case <-ctx.Done():
			l.Debug("Deployment cancelled")
			return
		default:
			deploymentErr = runDeployment(ctx, p)
		}
	}()

	_, err = prog.Run()
	if err != nil {
		l.Error(fmt.Sprintf("Error running program: %v", err))
		return err
	}

	// Clear the screen and print final table
	fmt.Print("\033[H\033[2J")
	fmt.Println(m.RenderFinalTable())

	return deploymentErr
}

func runDeployment(
	ctx context.Context,
	p azure.AzureProviderer,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	prog := display.GetGlobalProgram()

	// Prepare resource group
	l.Debug("Preparing resource group")
	err := p.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		return fmt.Errorf("failed to prepare resource group: %w", err)
	}
	l.Debug("Resource group prepared successfully")

	if err = p.DeployResources(ctx); err != nil {
		return fmt.Errorf("failed to deploy resources: %w", err)
	}

	if err := p.DeployOrchestrator(ctx); err != nil {
		return fmt.Errorf("failed to deploy Bacalhau orchestrator: %w", err)
	}

	for _, machine := range m.Deployment.Machines {
		if !machine.Orchestrator {
			if err := p.DeployWorker(ctx, machine.Name); err != nil {
				return fmt.Errorf("failed to deploy Bacalhau workers: %w", err)
			}
		}
	}

	if err := p.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	l.Info("Deployment finalized")
	time.Sleep(2 * time.Second)
	prog.Quit()

	return nil
}

func setDeploymentBasicInfo(d *models.Deployment) {
	d.SSHUser = viper.GetString("general.ssh_user")
	d.SSHPort = viper.GetInt("general.ssh_port")
	d.OrchestratorIP = viper.GetString("general.orchestrator_ip")
	d.ResourceGroupName = viper.GetString("azure.resource_group_name")
	d.ResourceGroupLocation = viper.GetString("azure.resource_group_location")
	d.AllowedPorts = viper.GetIntSlice("azure.allowed_ports")
	d.DefaultVMSize = viper.GetString("azure.default_vm_size")
	d.DefaultDiskSizeGB = int32(viper.GetInt("azure.default_disk_size_gb"))
	d.DefaultLocation = viper.GetString("azure.default_location")
	d.SubscriptionID = getSubscriptionID()
}

func setDeploymentDetails(d *models.Deployment, projectID, uniqueID string) {
	d.ProjectID = projectID
	d.UniqueID = uniqueID
	d.Tags = utils.EnsureAzureTags(make(map[string]*string), projectID, uniqueID)

	if d.ResourceGroupName == "" {
		d.ResourceGroupName = viper.GetString("azure.resource_group_name")
	}

	if d.Name == "" {
		d.Name = fmt.Sprintf("Azure Deployment - %s", uniqueID)
	}
}

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

	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read private key file: %w", err)
	}

	return publicKeyPath, privateKeyPath, strings.TrimSpace(
			string(publicKeyData),
		), strings.TrimSpace(
			string(privateKeyData),
		), nil
}

func extractSSHKeyPath(configKeyString string) (string, error) {
	keyPath := viper.GetString(configKeyString)
	if keyPath == "" {
		return "", fmt.Errorf("%s is empty", configKeyString)
	}

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

	if _, err := os.Stat(absoluteKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("key file does not exist: %s", absoluteKeyPath)
	}

	return absoluteKeyPath, nil
}

// We're going to use a temporary machine struct to unmarshal from the config file
// and then convert it to the actual machine struct
type rawMachine struct {
	Location   string `yaml:"location"`
	Parameters *struct {
		Count        int    `yaml:"count,omitempty"`
		Type         string `yaml:"type,omitempty"`
		Orchestrator bool   `yaml:"orchestrator,omitempty"`
	} `yaml:"parameters"`
}

func ProcessMachinesConfig(deployment *models.Deployment) error {
	l := logger.Get()
	locations := make(map[string]bool)

	rawMachines := []rawMachine{}

	if err := viper.UnmarshalKey("azure.machines", &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt("azure.default_count_per_zone")
	if defaultCount == 0 {
		l.Error("azure.default_count_per_zone is empty")
		return fmt.Errorf("azure.default_count_per_zone is empty")
	}
	defaultType := viper.GetString("azure.default_machine_type")
	if defaultType == "" {
		l.Error("azure.default_machine_type is empty")
		return fmt.Errorf("azure.default_machine_type is empty")
	}

	defaultDiskSize := viper.GetInt("azure.disk_size_gb")
	if defaultDiskSize == 0 {
		l.Error("azure.disk_size_gb is empty")
		return fmt.Errorf("azure.disk_size_gb is empty")
	}

	privateKeyBytes, err := readPrivateKey(deployment.SSHPrivateKeyPath)
	if err != nil {
		return err
	}

	orchestratorIP := deployment.OrchestratorIP
	var orchestratorLocations []string
	for _, rawMachine := range rawMachines {
		if rawMachine.Parameters != nil && rawMachine.Parameters.Orchestrator {
			// We're doing some checking here to make sure that the orchestrator node not
			// specified in a way that would result in multiple orchestrator nodes
			if rawMachine.Parameters.Count == 0 {
				rawMachine.Parameters.Count = defaultCount
			}

			for i := 0; i < rawMachine.Parameters.Count; i++ {
				orchestratorLocations = append(orchestratorLocations, rawMachine.Location)
			}
		}
	}

	if len(orchestratorLocations) > 1 {
		return fmt.Errorf("multiple orchestrator nodes found")
	}

	type badMachineLocationCombo struct {
		location string
		vmSize   string
	}
	var allBadMachineLocationCombos []badMachineLocationCombo
	newMachines := make(map[string]*models.Machine)
	for _, rawMachine := range rawMachines {
		count := 1
		if rawMachine.Parameters != nil {
			if rawMachine.Parameters.Count > 0 {
				count = rawMachine.Parameters.Count
			}
		}

		var thisVMType string
		if rawMachine.Parameters != nil {
			thisVMType = rawMachine.Parameters.Type
			if thisVMType == "" {
				thisVMType = defaultType
			}
		}
		azureClient, err := azure.NewAzureClientFunc(deployment.SubscriptionID)
		if err != nil {
			return fmt.Errorf("failed to create Azure client: %w", err)
		}

		fmt.Printf("Validating machine type %s in location %s...", thisVMType, rawMachine.Location)
		valid, err := azureClient.ValidateMachineType(
			context.Background(),
			rawMachine.Location,
			thisVMType,
		)
		if !valid || err != nil {
			allBadMachineLocationCombos = append(
				allBadMachineLocationCombos,
				badMachineLocationCombo{
					location: rawMachine.Location,
					vmSize:   thisVMType,
				},
			)
			fmt.Println("❌")
			continue
		}
		fmt.Println("✅")

		countOfMachines := getCountOfMachines(count, defaultCount)
		for i := 0; i < countOfMachines; i++ {
			newMachine, err := createNewMachine(
				rawMachine.Location,
				int32(defaultDiskSize),
				thisVMType,
				privateKeyBytes,
				deployment.SSHPort,
			)
			if err != nil {
				return fmt.Errorf("failed to create new machine: %w", err)
			}

			if rawMachine.Parameters != nil {
				if rawMachine.Parameters.Type != "" {
					newMachine.VMSize = rawMachine.Parameters.Type
				}
				if rawMachine.Parameters.Orchestrator {
					newMachine.Orchestrator = true
				}
			} else {
				// Log a warning or handle the case where Parameters is nil
				logger.Get().Warnf("Parameters for machine in location %s is nil", rawMachine.Location)
			}

			newMachines[newMachine.Name] = newMachine
			newMachines[newMachine.Name].SetResourceState(
				models.AzureResourceTypeVM.ResourceString,
				models.AzureResourceStateNotStarted,
			)
		}

		locations[rawMachine.Location] = true
	}

	if len(allBadMachineLocationCombos) > 0 {
		return fmt.Errorf(
			"invalid machine type and location combinations: %v",
			allBadMachineLocationCombos,
		)
	}

	// Loop for setting the orchestrator node
	orchestratorFound := false
	for name, machine := range newMachines {
		if orchestratorIP != "" {
			newMachines[name].OrchestratorIP = orchestratorIP
			orchestratorFound = true
		} else if machine.Orchestrator {
			orchestratorFound = true
		}
	}
	if !orchestratorFound {
		return fmt.Errorf("no orchestrator node and orchestratorIP is not set")
	}

	deployment.Machines = newMachines
	for k := range locations {
		deployment.Locations = append(deployment.Locations, k)
	}

	return nil
}

func readPrivateKey(path string) ([]byte, error) {
	privateKeyFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open private key file: %w", err)
	}
	defer privateKeyFile.Close()

	privateKeyBytes, err := io.ReadAll(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	if _, err = ssh.ParsePrivateKey(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKeyBytes, nil
}

func validateMachineOrchestrator(
	machine *models.Machine,
	orchestratorNode *models.Machine,
	deployment *models.Deployment,
) error {
	if machine.Orchestrator {
		if orchestratorNode != nil {
			return fmt.Errorf("multiple orchestrator nodes found")
		}
		if deployment.OrchestratorIP != "" {
			return fmt.Errorf("orchestrator node and deployment.OrchestratorIP cannot both be set")
		}
		return nil
	}

	if deployment.OrchestratorIP != "" && orchestratorNode != nil {
		return fmt.Errorf("orchestrator node and deployment.OrchestratorIP cannot both be set")
	}

	return nil
}

func getCountOfMachines(paramCount, defaultCount int) int {
	if paramCount == 0 {
		if defaultCount == 0 {
			return 1
		}
		return defaultCount
	}
	return paramCount
}

func createNewMachine(
	location string,
	diskSizeGB int32,
	vmSize string,
	privateKeyBytes []byte,
	sshPort int,
) (*models.Machine, error) {
	newMachine, err := models.NewMachine(location, vmSize, diskSizeGB)
	if err != nil {
		return nil, fmt.Errorf("failed to create new machine: %w", err)
	}

	if err := newMachine.EnsureMachineServices(); err != nil {
		logger.Get().Errorf("Failed to ensure machine services: %v", err)
	}

	for _, service := range models.RequiredServices {
		newMachine.SetServiceState(service.Name, models.ServiceStateNotStarted)
	}

	newMachine.SSHUser = "azureuser"
	newMachine.SSHPort = sshPort
	newMachine.SSHPrivateKeyMaterial = privateKeyBytes

	return newMachine, nil
}

// Additional helper functions

func validateResourceGroup(deployment *models.Deployment) error {
	if deployment.ResourceGroupLocation == "" {
		return fmt.Errorf("resource group location is empty")
	}
	if deployment.ResourceGroupName == "" {
		return fmt.Errorf("resource group name is empty")
	}
	return nil
}

func validateAllowedPorts(deployment *models.Deployment) error {
	if len(deployment.AllowedPorts) == 0 {
		return fmt.Errorf("no allowed ports found in configuration")
	}
	return nil
}

func initializeAzureProvider() (azure.AzureProviderer, error) {
	azureProvider, err := azure.AzureProviderFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Azure provider: %w", err)
	}
	return azureProvider, nil
}

func validateDeployment(deployment *models.Deployment) error {
	if err := validateResourceGroup(deployment); err != nil {
		return err
	}
	if err := validateAllowedPorts(deployment); err != nil {
		return err
	}
	return nil
}

// PrepareDeployment prepares the deployment by setting up the resource group and initial configuration.
func PrepareDeployment(
	ctx context.Context,
	projectID string,
	uniqueID string,
) (*models.Deployment, error) {
	l := logger.Get()
	l.Debug("Starting PrepareDeployment")

	deployment := &models.Deployment{}
	setDeploymentBasicInfo(deployment)

	// Set the start time for the deployment
	deployment.StartTime = time.Now()
	l.Debugf("Deployment start time: %v", deployment.StartTime)

	var err error
	deployment.SSHPublicKeyPath,
		deployment.SSHPrivateKeyPath,
		deployment.SSHPublicKeyMaterial,
		deployment.SSHPrivateKeyMaterial,
		err = ExtractSSHKeyPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to extract SSH keys: %w", err)
	}

	if err := sshutils.ValidateSSHKeysFromPath(deployment.SSHPublicKeyPath, deployment.SSHPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %w", err)
	}

	setDeploymentDetails(deployment, projectID, uniqueID)

	// Ensure we have a location set
	if deployment.ResourceGroupLocation == "" {
		deployment.ResourceGroupLocation = "eastus" // Default Azure region
		l.Warn("No resource group location specified, using default: eastus")
	}

	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %w", err)
	}

	if err := ProcessMachinesConfig(deployment); err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	return deployment, nil
}

func setDefaultConfigurations() {
	viper.SetDefault("general.project_id", "default-project")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", getDefaultLogLevel())
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("general.ssh_user", "azureuser")
	viper.SetDefault("general.ssh_port", 22)
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
}

func getDefaultLogLevel() string {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		return "info"
	}
	return strings.ToLower(logLevel)
}
