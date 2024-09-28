package gcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
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

func ExecuteCreateDeployment(cmd *cobra.Command, _ []string) error {
	l := logger.Get()
	ctx := cmd.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if cancel != nil && ctx.Err() != nil {
			cancel()
		}
	}()

	configFile := viper.GetString("config")
	if configFile == "" {
		configFile = "./config.yaml"
	}

	configFile, err := filepath.Abs(configFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for config file: %w", err)
	}
	fmt.Println("Using config file:", configFile)
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}
	common.SetDefaultConfigurations("gcp")
	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return fmt.Errorf("gcp.organization_id is not set in the configuration")
	}

	billingAccountID := viper.GetString("gcp.billing_account_id")
	if billingAccountID == "" {
		return fmt.Errorf("gcp.billing_account_id is not set in the configuration")
	}

	// Initialize the GCP provider
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		organizationID,
		billingAccountID,
	)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to create GCP provider: %v", err))
		return fmt.Errorf("failed to create GCP provider: %w", err)
	}

	// Prepare the deployment
	deployment, err := gcpProvider.PrepareDeployment(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare deployment: %w", err)
	}

	m := display.NewDisplayModel(deployment)
	machines, locations, err := gcpProvider.ProcessMachinesConfig(ctx)
	if err != nil {
		if err.Error() == "no machines configuration found for provider gcp" {
			fmt.Println(
				`You can check the machine types available for a zone with the command: 
gcloud compute machine-types list --zones <ZONE> | jq -r '.[].name'`,
			)
			return nil
		}
		return fmt.Errorf("failed to process machines config: %w", err)
	}
	m.Deployment.SetMachines(machines)
	m.Deployment.SetLocations(locations)

	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	pollingErrChan := gcpProvider.StartResourcePolling(ctx)
	go func() {
		for err := range pollingErrChan {
			if err != nil {
				l.Error(fmt.Sprintf("Resource polling error: %v", err))
			}
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
			deploymentErr = runDeployment(ctx, gcpProvider)
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
	configFile = viper.ConfigFileUsed()
	if configFile == "" {
		l.Error("No configuration file found, could not write to file.")
		return nil
	}
	if err := viper.WriteConfigAs(configFile); err != nil {
		l.Error(fmt.Sprintf("Failed to write configuration to file: %v", err))
	} else {
		l.Info(fmt.Sprintf("Configuration written to %s", configFile))
	}

	// Always print the final table, even if we force quit
	fmt.Println(m.RenderFinalTable())

	if deploymentErr != nil {
		fmt.Println("Deployment failed, but configuration was written to file.")
		fmt.Println("The deployment error was:")
		fmt.Println(deploymentErr)
	}

	if err != nil {
		fmt.Println("General (unknown) error running program:")
		fmt.Println(err)
	}

	return nil
}

func runDeployment(ctx context.Context, gcpProvider *gcp_provider.GCPProvider) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	writeConfig := func() {
		configFile := viper.ConfigFileUsed()
		if configFile != "" {
			if err := viper.WriteConfigAs(configFile); err != nil {
				l.Error(fmt.Sprintf("Failed to write configuration to file: %v", err))
			} else {
				l.Debug(fmt.Sprintf("Configuration written to %s", configFile))
			}
		}
	}

	updateMachineConfig := func(machineName string) {
		machine := m.Deployment.GetMachine(machineName)
		if machine != nil {
			viper.Set(
				fmt.Sprintf(
					"deployments.%s.gcp.%s.%s",
					m.Deployment.UniqueID,
					m.Deployment.GCP.ProjectID,
					machineName,
				),
				models.MachineConfigToWrite(machine),
			)
			writeConfig()
		}
	}

	// Ensure project
	l.Debug("Ensuring project")
	err := gcpProvider.EnsureProject(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to ensure project: %v", err))
		return fmt.Errorf("failed to ensure project: %w", err)
	}
	writeConfig()

	// Enable required APIs
	l.Debug("Enabling required APIs")
	if err := gcpProvider.EnableRequiredAPIs(ctx); err != nil {
		l.Error(fmt.Sprintf("Failed to enable required APIs: %v", err))
		return fmt.Errorf("failed to enable required APIs: %w", err)
	}
	l.Debug("Required APIs enabled successfully")
	writeConfig()

	// Create resources
	if err := gcpProvider.CreateResources(ctx); err != nil {
		return fmt.Errorf("failed to create resources: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		updateMachineConfig(machine.GetName())
	}

	// Provision machines
	var wg sync.WaitGroup
	errChan := make(chan error, len(m.Deployment.Machines))

	for _, machine := range m.Deployment.Machines {
		wg.Add(1)
		go func(machineName string) {
			defer wg.Done()
			err := gcpProvider.GetClusterDeployer().ProvisionPackagesOnMachine(ctx, machineName)
			if err != nil {
				errChan <- fmt.Errorf("failed to provision machine %s: %w", machineName, err)
				return
			}
			machine := m.Deployment.GetMachine(machineName)
			machine.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateSucceeded)
			machine.SetServiceState(models.ServiceTypeDocker.Name, models.ServiceStateSucceeded)
			updateMachineConfig(machineName)
		}(machine.GetName())
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Provision Bacalhau cluster
	if err := gcpProvider.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateSucceeded)
		updateMachineConfig(machine.GetName())
	}

	// Finalize deployment
	if err := gcpProvider.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}

func init() {
	createGCPDeploymentCmd.Flags().String("config", "", "config file path")
	viper.BindPFlag("config", createGCPDeploymentCmd.Flags().Lookup("config"))
}
