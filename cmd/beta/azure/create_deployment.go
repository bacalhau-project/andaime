package azure

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	RetryTimeout   = 2 * time.Second
	DefaultSSHPort = 22
)

var configMu sync.Mutex

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
	ctx := cmd.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if cancel != nil && ctx.Err() != nil {
			cancel()
		}
	}()

	if err := initializeConfig(cmd); err != nil {
		return err
	}

	azureProvider, err := initializeAzureProvider(ctx)
	if err != nil {
		return err
	}

	deployment, err := prepareDeployment(ctx, azureProvider)
	if err != nil {
		return err
	}

	m := display.NewDisplayModel(deployment)
	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	go startResourcePolling(ctx, azureProvider)

	deploymentErr := runDeploymentAsync(ctx, azureProvider, cancel)

	handleDeploymentCompletion(ctx, m, deploymentErr)

	return nil
}

func initializeConfig(_ *cobra.Command) error {
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}
	common.SetDefaultConfigurations("azure")
	return nil
}

func initializeAzureProvider(ctx context.Context) (*azure.AzureProvider, error) {
	subscriptionID := viper.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription_id is not set in the configuration")
	}
	azureProvider, err := azure.NewAzureProviderFunc(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure provider: %w", err)
	}

	client, err := azure.NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}
	azureProvider.SetAzureClient(client)

	return azureProvider, nil
}

func prepareDeployment(
	ctx context.Context,
	azureProvider *azure.AzureProvider,
) (*models.Deployment, error) {
	deployment, err := azureProvider.PrepareDeployment(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}
	deployment.Azure.ResourceGroupName = azureProvider.ResourceGroupName

	machines, locations, err := azureProvider.ProcessMachinesConfig(ctx)
	if err != nil {
		if err.Error() == "no machines configuration found for provider azure" {
			fmt.Println(
				"You can check the skus available for a location with the command: az vm list-skus -o json -l <ZONE> | jq -r '.[].name'",
			)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to process machines config: %w", err)
	}
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	return deployment, nil
}

func startResourcePolling(
	ctx context.Context,
	azureProvider *azure.AzureProvider,
) {
	l := logger.Get()
	err := azureProvider.StartResourcePolling(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to start resource polling: %v", err))
	}
}

func runDeploymentAsync(
	ctx context.Context,
	azureProvider *azure.AzureProvider,
	cancel context.CancelFunc,
) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	deploymentDone := make(chan struct{})
	var deploymentErr error

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
				cancel()
			}
		}
	}()

	_, err := prog.Run()
	if err != nil {
		l.Error(fmt.Sprintf("Error running program: %v", err))
		cancel()
	}

	select {
	case <-deploymentDone:
	case <-ctx.Done():
		l.Debug("Context cancelled, waiting for deployment to finish")
		<-deploymentDone
	}

	return deploymentErr
}

func runDeployment(ctx context.Context, azureProvider *azure.AzureProvider) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	defer func() {
		fmt.Println(m.RenderFinalTable())
	}()

	if err := prepareResourceGroup(ctx, azureProvider); err != nil {
		return err
	}

	if err := createResources(ctx, azureProvider, m); err != nil {
		return err
	}

	if err := provisionBacalhauCluster(ctx, azureProvider, m); err != nil {
		return err
	}

	if err := finalizeDeployment(ctx, azureProvider); err != nil {
		return err
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}

func prepareResourceGroup(ctx context.Context, azureProvider *azure.AzureProvider) error {
	l := logger.Get()
	l.Debug("Preparing resource group")
	err := azureProvider.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		return fmt.Errorf("failed to prepare resource group: %w", err)
	}
	l.Debug("Resource group prepared successfully")
	writeConfig()
	return nil
}

func createResources(
	ctx context.Context,
	azureProvider *azure.AzureProvider,
	m *display.DisplayModel,
) error {
	if err := azureProvider.CreateResources(ctx); err != nil {
		return fmt.Errorf("failed to create resources: %w", err)
	}
	for _, machine := range m.Deployment.GetMachines() {
		updateMachineConfig(m.Deployment, machine.GetName())
	}
	return nil
}

func provisionBacalhauCluster(
	ctx context.Context,
	azureProvider *azure.AzureProvider,
	m *display.DisplayModel,
) error {
	if err := azureProvider.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}
	for _, machine := range m.Deployment.GetMachines() {
		updateMachineConfig(m.Deployment, machine.GetName())
	}
	return nil
}

func finalizeDeployment(ctx context.Context, azureProvider *azure.AzureProvider) error {
	if err := azureProvider.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}
	return nil
}

func writeConfig() {
	l := logger.Get()
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		if err := viper.WriteConfigAs(configFile); err != nil {
			l.Error(fmt.Sprintf("Failed to write configuration to file: %v", err))
		} else {
			l.Debug(fmt.Sprintf("Configuration written to %s", configFile))
		}
	}
}

func updateMachineConfig(deployment *models.Deployment, machineName string) {
	configMu.Lock()
	defer configMu.Unlock()

	machine := deployment.GetMachine(machineName)
	if machine != nil {
		viper.Set(
			fmt.Sprintf(
				"deployments.%s.azure.%s.%s",
				deployment.UniqueID,
				deployment.Azure.ResourceGroupName,
				machineName,
			),
			models.MachineConfigToWrite(machine),
		)
		writeConfig()
	}
}

func handleDeploymentCompletion(ctx context.Context, m *display.DisplayModel, deploymentErr error) {
	writeConfig()

	if os.Getenv("ANDAIME_TEST_MODE") != "true" {
		fmt.Print("\033[H\033[2J")
		fmt.Println(m.RenderFinalTable())
	}

	if deploymentErr != nil {
		fmt.Println("Deployment failed, but configuration was written to file.")
		fmt.Println("The deployment error was:")
		fmt.Println(deploymentErr)
	}

	if ctx.Err() != nil {
		fmt.Println("General (unknown) error running program:")
		fmt.Println(ctx.Err())
	}
}
