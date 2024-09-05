package gcp

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
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	RetryTimeout   = 2 * time.Second
	DefaultSSHPort = 22
)

var createGCPDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in GCP",
	Long:  `Create a deployment in GCP using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func GetGCPCreateDeploymentCmd() *cobra.Command {
	return createGCPDeploymentCmd
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()
	l.Info("Starting executeCreateDeployment for GCP")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	setDefaultConfigurations()

	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		return fmt.Errorf("project prefix is empty")
	}

	uniqueID := time.Now().Format("060102150405")
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, uniqueID)

	p, err := gcp.NewGCPProvider(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize GCP provider: %w", err)
	}

	viper.Set("general.unique_id", uniqueID)
	deployment, err := PrepareDeployment(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize deployment: %w", err)
	}

	m := display.InitialModel(deployment)
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
			deploymentErr = p.CreateResources(ctx)
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

func setDefaultConfigurations() {
	viper.SetDefault("general.project_prefix", "andaime")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", getDefaultLogLevel())
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("general.ssh_user", "andaime")
	viper.SetDefault("general.ssh_port", DefaultSSHPort)
	viper.SetDefault("gcp.project_id", "")
	viper.SetDefault("gcp.region", "us-central1")
	viper.SetDefault("gcp.zone", "us-central1-a")
	viper.SetDefault("gcp.machine_type", "e2-medium")
	viper.SetDefault("gcp.disk_size_gb", globals.DefaultDiskSizeGB)
	viper.SetDefault("gcp.allowed_ports", globals.DefaultAllowedPorts)
	viper.SetDefault("gcp.machines", []map[string]interface{}{
		{
			"name":     "default-vm",
			"vm_size":  viper.GetString("gcp.machine_type"),
			"location": viper.GetString("gcp.zone"),
			"parameters": map[string]interface{}{
				"orchestrator": true,
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

func PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	l := logger.Get()
	l.Debug("Starting PrepareDeployment for GCP")

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

	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %w", err)
	}

	if err := ProcessMachinesConfig(deployment); err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	return deployment, nil
}

func setDeploymentBasicInfo(deployment *models.Deployment) error {
	projectPrefix := viper.GetString("general.project_prefix")
	uniqueID := viper.GetString("general.unique_id")
	deployment.Name = fmt.Sprintf("%s-%s", projectPrefix, uniqueID)
	deployment.GCP.ProjectID = viper.GetString("gcp.project_id")
	deployment.GCP.OrganizationID = viper.GetString("gcp.organization_id")
	deployment.GCP.BillingAccountID = viper.GetString("gcp.billing_account_id")
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

	if err := viper.UnmarshalKey("gcp.machines", &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt("gcp.default_count_per_zone")
	if defaultCount == 0 {
		l.Error("gcp.default_count_per_zone is empty")
		return fmt.Errorf("gcp.default_count_per_zone is empty")
	}
	defaultType := viper.GetString("gcp.default_machine_type")
	if defaultType == "" {
		l.Error("gcp.default_machine_type is empty")
		return fmt.Errorf("gcp.default_machine_type is empty")
	}

	defaultDiskSize := viper.GetInt("gcp.disk_size_gb")
	if defaultDiskSize == 0 {
		l.Error("gcp.disk_size_gb is empty")
		return fmt.Errorf("gcp.disk_size_gb is empty")
	}

	orgID := viper.GetString("gcp.organization_id")
	if orgID == "" {
		l.Error("gcp.organization_id is empty")
		return fmt.Errorf("gcp.organization_id is empty")
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

		// Probably can't do this validation without a GCP client - skipping for now

		// gcpClient, cleanup, err := gcp.NewGCPClientFunc(ctx, orgID)
		// if err != nil {
		// 	return fmt.Errorf("failed to create GCP client: %w", err)
		// }
		// deployment.Cleanup = cleanup

		// fmt.Printf("Validating machine type %s in location %s...", thisVMType, rawMachine.Location)
		// valid, err := gcpClient.ValidateMachineType(
		// 	ctx,
		// 	rawMachine.Location,
		// 	thisVMType,
		// )
		// if !valid || err != nil {
		// 	allBadMachineLocationCombos = append(
		// 		allBadMachineLocationCombos,
		// 		badMachineLocationCombo{
		// 			location: rawMachine.Location,
		// 			vmSize:   thisVMType,
		// 		},
		// 	)
		// 	fmt.Println("❌")
		// 	continue
		// }
		// fmt.Println("✅")

		if !isValidGCPLocation(rawMachine.Location) {
			return fmt.Errorf("invalid location for GCP: %s", rawMachine.Location)
		}

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
				return fmt.Errorf("failed to create raw machine: %w", err)
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
				models.GCPResourceTypeVM.ResourceString,
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
	privateKeyBytes []byte,
	sshPort int,
) (*models.Machine, error) {
	newMachine, err := models.NewMachine(models.DeploymentTypeGCP, location, vmSize, diskSizeGB)
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
func isValidGCPLocation(location string) bool {
	// List of valid GCP regions and zones
	validLocations := map[string]bool{
		"us-central1":     true,
		"us-central1-a":   true,
		"us-central1-b":   true,
		"us-central1-c":   true,
		"us-central1-f":   true,
		"us-east1":        true,
		"us-east1-b":      true,
		"us-east1-c":      true,
		"us-east1-d":      true,
		"us-east4":        true,
		"us-east4-a":      true,
		"us-east4-b":      true,
		"us-east4-c":      true,
		"us-west1":        true,
		"us-west1-a":      true,
		"us-west1-b":      true,
		"us-west1-c":      true,
		"europe-west1":    true,
		"europe-west1-b":  true,
		"europe-west1-c":  true,
		"europe-west1-d":  true,
		"europe-west2":    true,
		"europe-west2-a":  true,
		"europe-west2-b":  true,
		"europe-west2-c":  true,
		"asia-east1":      true,
		"asia-east1-a":    true,
		"asia-east1-b":    true,
		"asia-east1-c":    true,
		"asia-southeast1": true,
		"asia-southeast1-a": true,
		"asia-southeast1-b": true,
		"asia-southeast1-c": true,
	}

	return validLocations[location]
}
