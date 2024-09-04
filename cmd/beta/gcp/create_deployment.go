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

	if err := processMachinesConfig(deployment); err != nil {
		return nil, fmt.Errorf("failed to process machine configurations: %w", err)
	}

	return deployment, nil
}

func setDeploymentBasicInfo(deployment *models.Deployment) error {
	projectPrefix := viper.GetString("general.project_prefix")
	uniqueID := viper.GetString("general.unique_id")
	deployment.Name = fmt.Sprintf("%s-%s", projectPrefix, uniqueID)
	deployment.ProjectID = viper.GetString("gcp.project_id")
	return nil
}

func processMachinesConfig(deployment *models.Deployment) error {
	machines := viper.Get("gcp.machines")
	machinesSlice, ok := machines.([]interface{})
	if !ok {
		return fmt.Errorf("invalid machines configuration")
	}

	for _, m := range machinesSlice {
		machine, ok := m.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid machine configuration")
		}

		newMachine := &models.Machine{
			Name:     machine["name"].(string),
			VMSize:   machine["vm_size"].(string),
			Location: machine["location"].(string),
			Parameters: models.Parameters{
				Orchestrator: machine["parameters"].(map[string]interface{})["orchestrator"].(bool),
			},
		}

		deployment.Machines[newMachine.Name] = newMachine
	}

	return nil
}
