package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	provider_common "github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	ctx := cmd.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if cancel != nil && ctx.Err() != nil {
			cancel()
		}
	}()

	subscriptionID := viper.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return fmt.Errorf("subscription_id is not set in the configuration")
	}
	// Initialize the Azure provider
	p, err := providers.GetProvider(ctx, models.DeploymentTypeAzure)
	if err != nil {
		return fmt.Errorf("failed to create Azure provider: %w", err)
	}
	if p == nil {
		return fmt.Errorf("azure provider is nil")
	}

	// Prepare the deployment
	deployment, err := p.PrepareDeployment(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare deployment: %w", err)
	}

	m := display.NewDisplayModel(deployment)
	err = azure_provider.ProcessMachinesConfig(ctx)
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
	p provider_common.Providerer,
) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	// Prepare resource group
	l.Debug("Preparing resource group")
	err := p.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		return fmt.Errorf("failed to prepare resource group: %w", err)
	}
	l.Debug("Resource group prepared successfully")

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

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}
