package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
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
	defaultType := viper.GetString("gcp.default_machine_type")
	defaultDiskSize := viper.GetInt("gcp.disk_size_gb")
	defaultZone := viper.GetString("gcp.default_zone")
	defaultSourceImage := viper.GetString("gcp.default_source_image")
	orgID := viper.GetString("gcp.organization_id")

	if orgID == "" {
		l.Error("gcp.organization_id is empty")
		return fmt.Errorf("gcp.organization_id is empty")
	}

	privateKeyBytes, err := sshutils.ReadPrivateKey(deployment.SSHPrivateKeyPath)
	if err != nil {
		return err
	}

	newMachines := make(map[string]*models.Machine)
	for _, rawMachine := range rawMachines {
		count := defaultCount
		if rawMachine.Parameters != nil && rawMachine.Parameters.Count > 0 {
			count = rawMachine.Parameters.Count
		}

		vmType := defaultType
		if rawMachine.Parameters != nil && rawMachine.Parameters.Type != "" {
			vmType = rawMachine.Parameters.Type
		}

		location := rawMachine.Location
		if location == "" {
			location = defaultZone
		}

		if !internal_gcp.IsValidGCPLocation(location) {
			return fmt.Errorf("invalid location for GCP: %s", location)
		}

		if !internal_gcp.IsValidGCPMachineType(location, vmType) {
			return fmt.Errorf("invalid machine type for GCP: %s", vmType)
		}

		for i := 0; i < count; i++ {
			newMachine, err := createNewMachine(
				location,
				int32(defaultDiskSize),
				vmType,
				privateKeyBytes,
				deployment.SSHPort,
			)
			if err != nil {
				return fmt.Errorf("failed to create raw machine: %w", err)
			}

			// Set the source image in the machine's Parameters
			if newMachine.Parameters == nil {
				newMachine.Parameters = make(map[string]interface{})
			}
			newMachine.Parameters["source_image"] = defaultSourceImage

			if rawMachine.Parameters != nil && rawMachine.Parameters.Orchestrator {
				newMachine.Orchestrator = true
			}

			newMachines[newMachine.Name] = newMachine
			newMachines[newMachine.Name].SetResourceState(
				"VM",
				models.ResourceStateNotStarted,
			)
		}

		locations[location] = true
	}

	// Set orchestrator if not explicitly set
	orchestratorFound := false
	for _, machine := range newMachines {
		if machine.Orchestrator {
			orchestratorFound = true
			break
		}
	}
	if !orchestratorFound && len(newMachines) > 0 {
		// Set the first machine as orchestrator
		for _, machine := range newMachines {
			machine.Orchestrator = true
			break
		}
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
