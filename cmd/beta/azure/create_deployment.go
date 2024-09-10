package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
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
	RunE:  ExecuteCreateDeployment,
}

func GetAzureCreateDeploymentCmd() *cobra.Command {
	return createAzureDeploymentCmd
}

func ExecuteCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()
	l.Info("Starting executeCreateDeployment")

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	common.SetDefaultConfigurations("azure")

	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		return fmt.Errorf("project prefix is empty")
	}

	uniqueID := time.Now().Format("060102150405")
	ctx = context.WithValue(ctx, globals.UniqueDeploymentIDKey, uniqueID)

	p, err := azure_provider.NewAzureProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	viper.Set("general.unique_id", uniqueID)
	deployment, err := common.PrepareDeployment(ctx, models.DeploymentTypeAzure)
	if err != nil {
		return fmt.Errorf("failed to initialize deployment: %w", err)
	}
	m := display.NewDisplayModel(deployment)
	err = azure_provider.ProcessMachinesConfig()
	if err != nil {
		return fmt.Errorf("failed to process machines config: %w", err)
	}

	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	go p.StartResourcePolling(ctx)

	var deploymentErr error
	deploymentDone := make(chan struct{})

	go func() {
		defer close(deploymentDone)
		select {
		case <-ctx.Done():
			l.Debug("Deployment cancelled")
			return
		default:
			deploymentErr = runDeployment(ctx, p)
			if deploymentErr != nil {
				l.Error(fmt.Sprintf("Deployment failed: %v", deploymentErr))
				cancel() // Cancel the context on error
			}
		}
	}()

	_, err = prog.Run()
	if err != nil {
		l.Error(fmt.Sprintf("Error running program: %v", err))
		cancel() // Cancel the context on error
	}

	// Wait for deployment to finish or context to be cancelled
	select {
	case <-deploymentDone:
	case <-ctx.Done():
		l.Debug("Context cancelled, waiting for deployment to finish")
		<-deploymentDone
	}

	// Write configuration to file
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		l.Error("No configuration file found, could not write to file.")
		return nil
	}
	if err := viper.WriteConfigAs(configFile); err != nil {
		l.Error(fmt.Sprintf("Failed to write configuration to file: %v", err))
	} else {
		l.Info(fmt.Sprintf("Configuration written to %s", configFile))
	}

	if os.Getenv("ANDAIME_TEST_MODE") != "true" { //nolint:goconst
		// Clear the screen and print final table
		fmt.Print("\033[H\033[2J")
		fmt.Println(m.RenderFinalTable())
	}

	if deploymentErr != nil {
		fmt.Println("Deployment failed, but configuration was written to file.")
		fmt.Println("The deployment error was:")
		fmt.Println(deploymentErr)
	}

	if err != nil {
		fmt.Println("General (unknown) error running program:")
		fmt.Println(err)
		return nil
	}

	return nil
}

func runDeployment(
	ctx context.Context,
	p azure_provider.AzureProviderer,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	prog := display.GetGlobalProgramFunc()

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
			if err := p.GetClusterDeployer().ProvisionOrchestrator(ctx, machine.Name); err != nil {
				return fmt.Errorf("failed to provision Bacalhau: %w", err)
			}
			break
		}
	}

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}
		eg.Go(func() error {
			if err := p.GetClusterDeployer().ProvisionWorker(ctx, machine.Name); err != nil {
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
