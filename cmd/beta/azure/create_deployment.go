package azure

import (
	"context"
	"fmt"
	"os"
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
	"golang.org/x/sync/errgroup"
)

const (
	RetryTimeout   = 2 * time.Second
	DefaultSSHPort = 22
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

	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		return fmt.Errorf("project prefix is empty")
	}

	uniqueID := time.Now().Format("060102150405")
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, uniqueID)

	p, err := azure.NewAzureProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	viper.Set("general.unique_id", uniqueID)
	deployment, err := PrepareDeployment(ctx)
	m := display.InitialModel(deployment)
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

	if err = p.CreateResources(ctx); err != nil {
		return fmt.Errorf("failed to deploy resources: %w", err)
	}

	// Go through each machine and set the resource state to succeeded - since it's
	// already been deployed (this is basically a UI bug fix)
	for i := range m.Deployment.Machines {
		for k := range m.Deployment.Machines[i].GetMachineResources() {
			m.Deployment.Machines[i].SetResourceState(
				k,
				models.ResourceStateSucceeded,
			)
		}
	}

	var eg errgroup.Group

	for _, machine := range m.Deployment.Machines {
		eg.Go(func() error {
			return p.GetClusterDeployer().ProvisionPackagesOnMachine(ctx, machine.Name)
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to provision packages on machines: %w", err)
	}

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			if err := p.GetClusterDeployer().DeployOrchestrator(ctx); err != nil {
				return fmt.Errorf("failed to provision Bacalhau: %w", err)
			}
			break
		}
	}

	for _, machine := range m.Deployment.Machines {
		eg.Go(func() error {
			if err := p.GetClusterDeployer().DeployWorker(ctx, machine.Name); err != nil {
				return fmt.Errorf("failed to configure Bacalhau: %w", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to deploy workers: %w", err)
	}
	if err := p.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}

func setDeploymentBasicInfo(d *models.Deployment) error {
	d.SSHUser = viper.GetString("general.ssh_user")
	d.SSHPort = viper.GetInt("general.ssh_port")
	d.OrchestratorIP = viper.GetString("general.orchestrator_ip")
	d.Azure.ResourceGroupName = viper.GetString("azure.resource_group_name")
	d.Azure.ResourceGroupLocation = viper.GetString("azure.resource_group_location")
	d.AllowedPorts = viper.GetIntSlice("azure.allowed_ports")
	d.Azure.DefaultVMSize = viper.GetString("azure.default_vm_size")

	defaultDiskSize := viper.GetInt("azure.default_disk_size_gb")
	d.Azure.DefaultDiskSizeGB = utils.GetSafeDiskSize(defaultDiskSize)
	d.Azure.DefaultLocation = viper.GetString("azure.default_location")
	subscriptionID, err := getSubscriptionID()
	if err != nil {
		return fmt.Errorf("failed to get subscription ID: %w", err)
	}
	d.Azure.SubscriptionID = subscriptionID
	return nil
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

//nolint:funlen,gocyclo,unused
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

	privateKeyBytes, err := sshutils.ReadPrivateKey(deployment.SSHPrivateKeyPath)
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
		azureClient, err := azure.NewAzureClientFunc(deployment.Azure.SubscriptionID)
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

		countOfMachines := utils.GetCountOfMachines(count, defaultCount)
		for i := 0; i < countOfMachines; i++ {
			newMachine, err := createNewMachine(
				rawMachine.Location,
				utils.GetSafeDiskSize(defaultDiskSize),
				thisVMType,
				deployment.SSHPrivateKeyPath,
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
				models.ResourceStateNotStarted,
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

func createNewMachine(
	location string,
	diskSizeGB int32,
	vmSize string,
	privateKeyPath string,
	privateKeyBytes []byte,
	sshPort int,
) (*models.Machine, error) {
	newMachine, err := models.NewMachine(models.DeploymentTypeAzure, location, vmSize, diskSizeGB)
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
	newMachine.SSHPrivateKeyPath = privateKeyPath
	newMachine.SSHPrivateKeyMaterial = privateKeyBytes

	return newMachine, nil
}

// PrepareDeployment prepares the deployment by setting up the resource group and initial configuration.
func PrepareDeployment(
	ctx context.Context,
) (*models.Deployment, error) {
	l := logger.Get()
	l.Debug("Starting PrepareDeployment")

	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		return nil, fmt.Errorf("general.project_prefix is not set")
	}
	uniqueID := viper.GetString("general.unique_id")
	if uniqueID == "" {
		return nil, fmt.Errorf("general.unique_id is not set")
	}
	deployment, err := models.NewDeployment()
	if err != nil {
		return nil, fmt.Errorf("failed to create new deployment: %w", err)
	}
	if err := setDeploymentBasicInfo(deployment); err != nil {
		return nil, fmt.Errorf("failed to set deployment basic info: %w", err)
	}

	// Set the start time for the deployment
	deployment.StartTime = time.Now()
	l.Debugf("Deployment start time: %v", deployment.StartTime)

	deployment.SSHPublicKeyPath,
		deployment.SSHPrivateKeyPath,
		deployment.SSHPublicKeyMaterial,
		deployment.SSHPrivateKeyMaterial,
		err = sshutils.ExtractSSHKeyPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to extract SSH keys: %w", err)
	}

	if err := sshutils.ValidateSSHKeysFromPath(deployment.SSHPublicKeyPath, deployment.SSHPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %w", err)
	}

	// Ensure we have a location set
	if deployment.Azure.ResourceGroupLocation == "" {
		deployment.Azure.ResourceGroupLocation = "eastus" // Default Azure region
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
	viper.SetDefault("general.project_prefix", "andaime")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", getDefaultLogLevel())
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("general.ssh_user", "azureuser")
	viper.SetDefault("general.ssh_port", DefaultSSHPort)
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
