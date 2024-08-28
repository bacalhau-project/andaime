package azure

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	uniqueID := time.Now().Format("060102150405")
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, uniqueID)

	p, err := azure.AzureProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	deployment, err := InitializeDeployment(ctx, uniqueID)
	if err != nil {
		return fmt.Errorf("failed to initialize deployment: %w", err)
	}

	m := display.InitialModel()
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
			deploymentErr = runDeployment(ctx, p, deployment)
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
	deployment *models.Deployment,
) error {
	l := logger.Get()
	prog := display.GetGlobalProgram()

	if err := p.DeployResources(ctx); err != nil {
		return fmt.Errorf("failed to deploy resources: %w", err)
	}

	if err := p.DeployOrchestrator(ctx); err != nil {
		for _, machine := range deployment.Machines {
			machine.SetResourceState(
				models.AzureResourceTypeVM.ResourceString,
				models.AzureResourceStateFailed,
			)
		}
		return fmt.Errorf("failed to deploy Bacalhau orchestrator: %w", err)
	}

	updateOrchestratorIP(deployment)

	numberOfSimultaneousProvisionings := models.NumberOfSimultaneousProvisionings
	workerChan := make(chan string, len(deployment.Machines))
	errChan := make(chan error, len(deployment.Machines))
	var wg sync.WaitGroup

	// Fill the channel with worker names
	for _, machine := range deployment.Machines {
		if !machine.Orchestrator {
			workerChan <- machine.Name
		}
	}
	close(workerChan)

	// Start worker deployments
	for i := 0; i < numberOfSimultaneousProvisionings; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for workerName := range workerChan {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
					if err := p.DeployWorker(ctx, workerName); err != nil {
						errChan <- fmt.Errorf("failed to deploy Bacalhau worker %s: %w", workerName, err)
						return
					}
				}
			}
		}()
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("error deploying workers: %w", err)
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

func updateOrchestratorIP(deployment *models.Deployment) {
	for i := range deployment.Machines {
		deployment.Machines[i].OrchestratorIP = deployment.OrchestratorIP
	}
}

func InitializeDeployment(ctx context.Context, uniqueID string) (*models.Deployment, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("deployment cancelled before starting: %w", err)
	}

	setDefaultConfigurations()

	projectID := viper.GetString("general.project_id")
	return PrepareDeployment(ctx, projectID, uniqueID)
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

func PrepareDeployment(
	ctx context.Context,
	projectID, uniqueID string,
) (*models.Deployment, error) {
	deployment := &models.Deployment{}
	setDeploymentBasicInfo(deployment)

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

	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %w", err)
	}

	if err := ProcessMachinesConfig(deployment); err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	return deployment, nil
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
	Parameters struct {
		Count        int    `yaml:"count,omitempty"`
		Type         string `yaml:"type,omitempty"`
		Orchestrator bool   `yaml:"orchestrator,omitempty"`
	} `yaml:"parameters"`
}

func ProcessMachinesConfig(deployment *models.Deployment) error {
	locations := make(map[string]bool)

	rawMachines := []rawMachine{}

	if err := viper.UnmarshalKey("azure.machines", &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt("azure.default_count_per_zone")
	defaultType := viper.GetString("azure.default_machine_type")
	defaultDiskSize := viper.GetInt("azure.disk_size_gb")

	privateKeyBytes, err := readPrivateKey(deployment.SSHPrivateKeyPath)
	if err != nil {
		return err
	}

	orchestratorIP := deployment.OrchestratorIP

	var orchestratorLocations []string
	for _, rawMachine := range rawMachines {
		if rawMachine.Parameters.Orchestrator {
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

	newMachines := make(map[string]*models.Machine)
	for _, rawMachine := range rawMachines {
		countOfMachines := getCountOfMachines(rawMachine.Parameters.Count, defaultCount)
		for i := 0; i < countOfMachines; i++ {
			diskSizeGB := defaultDiskSize
			vmSize := defaultType

			if rawMachine.Parameters.Type != "" {
				vmSize = rawMachine.Parameters.Type
			}

			newMachine := createNewMachine(
				rawMachine.Location,
				int32(diskSizeGB),
				vmSize,
				privateKeyBytes,
				deployment.SSHPort,
			)
			newMachines[newMachine.Name] = newMachine
		}

		locations[rawMachine.Location] = true
	}

	// Loop for setting the orchestrator node
	orchestratorFound := false
	for name := range newMachines {
		if orchestratorIP != "" {
			newMachines[name].OrchestratorIP = orchestratorIP
			orchestratorFound = true
		} else if len(orchestratorLocations) > 0 && newMachines[name].Location == orchestratorLocations[0] {
			newMachines[name].Orchestrator = true
			orchestratorFound = true
		} else {
			newMachines[name].Orchestrator = false
		}
	}
	if !orchestratorFound {
		return fmt.Errorf("no orchestrator node and orchestratorIP is not set")
	}

	deployment.Machines = newMachines
	deployment.UniqueLocations = getUniqueLocations(locations)

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
) *models.Machine {
	newMachine := models.NewMachine(location, vmSize, diskSizeGB)

	if err := newMachine.EnsureMachineServices(); err != nil {
		logger.Get().Errorf("Failed to ensure machine services: %v", err)
	}

	for _, service := range models.RequiredServices {
		newMachine.SetServiceState(service.Name, models.ServiceStateNotStarted)
	}

	newMachine.SSHUser = "azureuser"
	newMachine.SSHPort = sshPort
	newMachine.SSHPrivateKeyMaterial = privateKeyBytes

	return newMachine
}

func getUniqueLocations(locations map[string]bool) []string {
	uniqueLocations := make([]string, 0, len(locations))
	for location := range locations {
		uniqueLocations = append(uniqueLocations, location)
	}
	return uniqueLocations
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

func initializeDisplayModel(deployment *models.Deployment) *display.DisplayModel {
	displayModel := display.InitialModel()
	displayModel.Deployment = deployment
	return displayModel
}
