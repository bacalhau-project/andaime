package gcp

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/providers/factory"
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
	RunE:  ExecuteCreateDeployment,
}

func GetGCPCreateDeploymentCmd() *cobra.Command {
	return createGCPDeploymentCmd
}

func ExecuteCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()
	l.Info("Starting executeCreateDeployment for GCP")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	common.SetDefaultConfigurations("gcp")

	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		return fmt.Errorf("project prefix is empty")
	}

	provider, err := factory.GetProvider(ctx, models.DeploymentTypeGCP)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	deployment, err := provider.PrepareDeployment(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare deployment: %w", err)
	}

	m := display.NewDisplayModel(deployment)
	machines, locations, err := common.ProcessMachinesConfig(
		deployment.DeploymentType,
		func(ctx context.Context, machineName string, machineType string) (bool, error) {
			return true, nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to process machines config: %w", err)
	}
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	go provider.StartResourcePolling(ctx)

	deploymentErr := runDeployment(ctx, provider)
	if deploymentErr != nil {
		l.Error(fmt.Sprintf("Deployment failed: %v", deploymentErr))
		cancel()
	}

	_, err = prog.Run()
	if err != nil {
		l.Error(fmt.Sprintf("Error running program: %v", err))
		return err
	}

	if os.Getenv("ANDAIME_TEST_MODE") != "true" { //nolint:goconst
		// Clear the screen and print final table
		fmt.Print("\033[H\033[2J")
		fmt.Println(m.RenderFinalTable())
	}

	return deploymentErr
}

func runDeployment(ctx context.Context, p common.Providerer) error {
	// Create resources
	if err := p.CreateResources(ctx); err != nil {
		return fmt.Errorf("failed to create resources: %w", err)
	}

	// Provision machines
	if err := p.GetClusterDeployer().ProvisionAllMachinesWithPackages(ctx); err != nil {
		return fmt.Errorf("failed to provision machines: %w", err)
	}

	// Provision Bacalhau cluster
	if err := p.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}

	// Finalize deployment
	if err := p.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	return nil
}
