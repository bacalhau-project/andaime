package gcp

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
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

func ExecuteCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()
	ctx := cmd.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if cancel != nil && ctx.Err() != nil {
			cancel()
		}
	}()

	common.SetDefaultConfigurations("gcp")

	projectID := viper.GetString("gcp.project_id")
	if projectID == "" {
		return fmt.Errorf("gcp.project_id is not set in the configuration")
	}

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
		projectID,
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
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	// Ensure project
	l.Debug("Ensuring project")
	projectID, err := gcpProvider.EnsureProject(ctx, m.Deployment.ProjectID)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to ensure project: %v", err))
		return fmt.Errorf("failed to ensure project: %w", err)
	}
	m.Deployment.ProjectID = projectID
	l.Debug("Project ensured successfully")

	// Enable required APIs
	l.Debug("Enabling required APIs")
	if err := gcpProvider.EnableRequiredAPIs(ctx); err != nil {
		l.Error(fmt.Sprintf("Failed to enable required APIs: %v", err))
		return fmt.Errorf("failed to enable required APIs: %w", err)
	}
	l.Debug("Required APIs enabled successfully")

	// Create resources
	if err := gcpProvider.CreateResources(ctx); err != nil {
		return fmt.Errorf("failed to create resources: %w", err)
	}
	writeConfig()

	// Provision machines
	if err := gcpProvider.GetClusterDeployer().ProvisionAllMachinesWithPackages(ctx); err != nil {
		return fmt.Errorf("failed to provision machines: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		machine.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateSucceeded)
		machine.SetServiceState(models.ServiceTypeDocker.Name, models.ServiceStateSucceeded)
	}
	writeConfig()

	// Provision Bacalhau cluster
	if err := gcpProvider.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateSucceeded)
	}
	writeConfig()

	// Finalize deployment
	if err := gcpProvider.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}
