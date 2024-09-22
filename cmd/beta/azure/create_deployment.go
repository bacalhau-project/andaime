package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
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
	azureProvider, err := azure.NewAzureProvider(ctx, subscriptionID)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to create Azure provider: %v", err))
		return fmt.Errorf("failed to create Azure provider: %w", err)
	}

	// Now you can use Azure-specific methods
	err = azureProvider.CreateResources(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to create Azure resources: %v", err))
		return fmt.Errorf("failed to create Azure resources: %w", err)
	}

	// Prepare the deployment
	deployment, err := azureProvider.PrepareDeployment(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare deployment: %w", err)
	}

	m := display.NewDisplayModel(deployment)
	machines, locations, err := azureProvider.ProcessMachinesConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process machines config: %w", err)
	}
	m.Deployment.SetMachines(machines)
	m.Deployment.SetLocations(locations)

	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	go func() {
		defer cancel()
		err = azureProvider.StartResourcePolling(ctx)
		if err != nil {
			l.Error(fmt.Sprintf("Failed to start resource polling: %v", err))
		}
	}()

	var deploymentErr error
	deploymentDone := make(chan struct{})

	go func() {
		defer close(deploymentDone)
		select {
		case <-ctx.Done():
			l.Debug("Deployment cancelled")
			return
		default:
			deploymentErr = runDeployment(ctx, azureProvider)
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
	azureProvider *azure_provider.AzureProvider,
) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	// Prepare resource group
	l.Debug("Preparing resource group")
	err := azureProvider.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		return fmt.Errorf("failed to prepare resource group: %w", err)
	}
	l.Debug("Resource group prepared successfully")

	// Create resources
	if err := azureProvider.CreateResources(ctx); err != nil {
		return fmt.Errorf("failed to create resources: %w", err)
	}

	// Provision machines
	if err := azureProvider.GetClusterDeployer().ProvisionAllMachinesWithPackages(ctx); err != nil {
		return fmt.Errorf("failed to provision machines: %w", err)
	}

	// Provision Bacalhau cluster
	if err := azureProvider.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}

	// Finalize deployment
	if err := azureProvider.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}
