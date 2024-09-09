package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/common"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
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

	uniqueID := time.Now().Format("0601021504")
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, uniqueID)

	p, err := gcp.NewGCPProvider(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize GCP provider: %w", err)
	}

	deployment, err := common.PrepareDeployment(ctx, "gcp")
	if err != nil {
		return fmt.Errorf("failed to prepare deployment: %w", err)
	}

	m := display.InitialModel(deployment)

	// Check permissions before starting deployment
	// if err := p.GetGCPClient().CheckPermissions(ctx); err != nil {
	// 	l.Error(fmt.Sprintf("Permission check failed: %v", err))
	// 	return fmt.Errorf("insufficient permissions to deploy: %w", err)
	// }

	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	go p.StartResourcePolling(ctx)

	go func() {
		deploymentErr := p.CreateResources(ctx)
		if deploymentErr != nil {
			l.Error(fmt.Sprintf("Error creating resources: %v", deploymentErr))
			cancel()
		}
		err := p.GetClusterDeployer().
			WaitForAllMachinesToReachState(ctx,
				models.GCPResourceTypeInstance.ResourceString,
				models.ResourceStateSucceeded,
			)
		if err != nil {
			l.Error(fmt.Sprintf("Error waiting for all machines to reach state: %v", err))
			cancel()
		}

		var dockerErrGroup errgroup.Group
		for _, machine := range deployment.Machines {
			internalMachine := machine
			dockerErrGroup.Go(func() error {
				return internalMachine.InstallDockerAndCorePackages(ctx)
			})
		}
		if err := dockerErrGroup.Wait(); err != nil {
			l.Error(fmt.Sprintf("Error installing Docker and core packages: %v", err))
			cancel()
		}

		var bacalhauErrGroup errgroup.Group
		bacalhauErrGroup.Go(func() error {
			return p.GetClusterDeployer().ProvisionBacalhau(ctx)
		})
		if err := bacalhauErrGroup.Wait(); err != nil {
			l.Error(fmt.Sprintf("Error installing Bacalhau: %v", err))
			cancel()
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

	return nil
}
