package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

const DefaultSSHPort = 22
const RetryTimeout = 10 * time.Second

func GetCreateDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-deployment",
		Short: "Create a new deployment in GCP",
		Long:  `Create a new deployment in Google Cloud Platform (GCP).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateDeployment(cmd, args)
		},
	}

	return cmd
}

func runCreateDeployment(cmd *cobra.Command, _ []string) error {
	l := logger.Get()
	l.Info("Creating deployment in GCP...")

	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return fmt.Errorf("organization_id is required")
	}

	p, err := gcp.NewGCPProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to create GCP provider: %w", err)
	}

	setDefaultConfigurations()

	d, err := models.NewDeployment()
	m := display.InitialModel(d)
	if err != nil {
		return fmt.Errorf("failed to create new deployment: %w", err)
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, d.UniqueID)

	d.Labels = utils.EnsureGCPLabels(make(map[string]string), d.ProjectID, d.UniqueID)

	if d.ProjectID == "" {
		d.ProjectID = viper.GetString("general.project_prefix")
	}

	if d.Name == "" {
		d.Name = fmt.Sprintf("GCP Deployment - %s", d.UniqueID)
	}
	m.Deployment = d

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
	p gcp.GCPProviderer,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	prog := display.GetGlobalProgram()

	// Prepare resource group
	l.Debug("Ensuring project")
	projectIDUpdated, err := p.EnsureProject(ctx, m.Deployment.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to ensure project: %w", err)
	}
	m.Deployment.ProjectID = projectIDUpdated

	if err = p.DeployResources(ctx); err != nil {
		return fmt.Errorf("failed to deploy resources: %w", err)
	}

	// Go through each machine and set the resource state to succeeded - since it's
	// already been deployed (this is basically a UI bug fix)
	for i := range m.Deployment.Machines {
		for k := range m.Deployment.Machines[i].GetMachineResources() {
			m.Deployment.Machines[i].SetResourceState(
				k,
				models.AzureResourceStateSucceeded,
			)
		}
	}

	if err = p.ProvisionPackagesOnMachines(ctx); err != nil {
		return fmt.Errorf("failed to provision machines: %w", err)
	}

	if err := p.ProvisionBacalhau(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau: %w", err)
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
	d.OrganizationID = viper.GetString("gcp.organization_id")
	d.ProjectID = viper.GetString("general.project_prefix")
	d.AllowedPorts = viper.GetIntSlice("gcp.allowed_ports")
	d.DefaultVMSize = viper.GetString("gcp.default_vm_size")

	defaultDiskSize := viper.GetInt("gcp.default_disk_size_gb")
	d.DefaultDiskSizeGB = utils.GetSafeDiskSize(defaultDiskSize)
	d.DefaultLocation = viper.GetString("gcp.default_location")
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

		countOfMachines := utils.GetCountOfMachines(count, defaultCount)
		for i := 0; i < countOfMachines; i++ {
			newMachine, err := createNewMachine(
				rawMachine.Location,
				utils.GetSafeDiskSize(defaultDiskSize),
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

// PrepareDeployment prepares the deployment by setting up the resource group and initial configuration.
func PrepareDeployment(
	ctx context.Context,
) (*models.Deployment, error) {
	l := logger.Get()
	l.Debug("Starting PrepareDeployment")

	deployment, err := models.NewDeployment()
	if err != nil {
		return nil, fmt.Errorf("failed to create new deployment: %w", err)
	}
	if err = setDeploymentBasicInfo(deployment); err != nil {
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

	deployment.Labels = utils.EnsureGCPLabels(
		make(map[string]string),
		deployment.ProjectID,
		deployment.UniqueID,
	)

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
	viper.SetDefault("general.project_prefix", "andaime")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", getDefaultLogLevel())
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("general.ssh_user", "azureuser")
	viper.SetDefault("general.ssh_port", DefaultSSHPort)
	viper.SetDefault("general.project_prefix", "andaime-project")
	viper.SetDefault("gcp.project_name", "andaime-project")
	viper.SetDefault("gcp.allowed_ports", globals.DefaultAllowedPorts)
	viper.SetDefault("gcp.default_vm_size", "n4-standard-4")
	viper.SetDefault("gcp.default_disk_size_gb", globals.DefaultDiskSizeGB)
	viper.SetDefault("gcp.default_location", "us-east1")
	viper.SetDefault("gcp.machines", []models.Machine{
		{
			Name:     "default-vm",
			VMSize:   viper.GetString("gcp.default_vm_size"),
			Location: viper.GetString("gcp.default_location"),
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
