package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
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
	ctx := cmd.Context()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	gcpProvider, err := initializeGCPProvider(ctx)
	if err != nil {
		return err
	}

	deployment, err := prepareDeployment(ctx, gcpProvider)
	if err != nil {
		return err
	}

	gcpProvider.ProjectID = deployment.GetProjectID()

	m := display.NewDisplayModel(deployment)
	prog := display.GetGlobalProgramFunc()
	err = prog.InitProgram(m)
	if err != nil {
		return err
	}

	go startResourcePolling(ctx, gcpProvider)

	deploymentErr := runDeploymentAsync(ctx, gcpProvider, cancel)

	handleDeploymentCompletion(ctx, m, deploymentErr)

	return nil
}

func initializeGCPProvider(ctx context.Context) (*gcp_provider.GCPProvider, error) {
	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is not set in the configuration")
	}

	billingAccountID := viper.GetString("gcp.billing_account_id")
	if billingAccountID == "" {
		return nil, fmt.Errorf("gcp.billing_account_id is not set in the configuration")
	}

	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		organizationID,
		billingAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP provider: %w", err)
	}

	return gcpProvider, nil
}

func prepareDeployment(
	ctx context.Context,
	gcpProvider *gcp_provider.GCPProvider,
) (*models.Deployment, error) {
	deployment, err := gcpProvider.PrepareDeployment(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	machines, locations, err := gcpProvider.ProcessMachinesConfig(ctx)
	if err != nil {
		if err.Error() == "no machines configuration found for provider gcp" {
			fmt.Println(
				`You can check the machine types available for a zone with the command: 
gcloud compute machine-types list --zones <ZONE> | jq -r '.[].name'`,
			)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to process machines config: %w", err)
	}
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	return deployment, nil
}

func startResourcePolling(ctx context.Context, gcpProvider *gcp_provider.GCPProvider) <-chan error {
	pollingErrChan := gcpProvider.StartResourcePolling(ctx)
	go func() {
		for err := range pollingErrChan {
			if err != nil {
				logger.Get().Error(fmt.Sprintf("Resource polling error: %v", err))
			}
		}
	}()
	return pollingErrChan
}

func runDeploymentAsync(
	ctx context.Context,
	gcpProvider *gcp_provider.GCPProvider,
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
			deploymentErr = runDeployment(ctx, gcpProvider)
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
		return err
	}

	select {
	case <-deploymentDone:
	case <-ctx.Done():
		l.Debug("Context cancelled, waiting for deployment to finish")
		<-deploymentDone
	}

	return deploymentErr
}

func runDeployment(ctx context.Context, gcpProvider *gcp_provider.GCPProvider) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	if err := ensureProject(ctx, gcpProvider); err != nil {
		return err
	}

	// Enable required APIs before starting resource polling
	l.Info("Enabling required APIs...")
	if err := gcpProvider.EnableRequiredAPIs(ctx); err != nil {
		l.Error(fmt.Sprintf("Failed to enable required APIs: %v", err))
		return err
	}
	l.Info("Required APIs enabled successfully")

	if err := provisionNetwork(ctx, gcpProvider); err != nil {
		return err
	}

	if err := createResources(ctx, gcpProvider); err != nil {
		return err
	}

	if err := provisionBacalhauCluster(ctx, gcpProvider); err != nil {
		return err
	}

	if err := finalizeDeployment(ctx, gcpProvider); err != nil {
		return err
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}

func ensureProject(ctx context.Context, gcpProvider *gcp_provider.GCPProvider) error {
	l := logger.Get()
	l.Debug("Ensuring project")
	err := gcpProvider.EnsureProject(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to ensure project: %v", err))
		return fmt.Errorf("failed to ensure project: %w", err)
	}
	writeConfig()
	return nil
}

func provisionNetwork(
	ctx context.Context,
	gcpProvider *gcp_provider.GCPProvider,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	networkName := fmt.Sprintf("%s-net", gcpProvider.ProjectID)
	l.Infof("Creating VPC network: %s", networkName)

	for _, machine := range m.Deployment.GetMachines() {
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machine.GetName(),
			models.GCPResourceTypeVPC,
			models.ResourceStatePending,
			fmt.Sprintf("Creating VPC network %s", networkName),
		))
	}

	if err := gcpProvider.CreateVPCNetwork(ctx, networkName); err != nil {
		l.Error(fmt.Sprintf("Failed to create VPC network: %v", err))
		for _, machine := range m.Deployment.GetMachines() {
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeVPC,
				models.ResourceStateFailed,
				fmt.Sprintf("Failed to create VPC network: %v", err),
			))
		}
		return fmt.Errorf("failed to create VPC network: %w", err)
	}

	for _, machine := range m.Deployment.GetMachines() {
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machine.GetName(),
			models.GCPResourceTypeVPC,
			models.ResourceStateSucceeded,
			fmt.Sprintf("Created VPC network %s", networkName),
		))
	}

	l.Info("Creating firewall rules...")
	if err := gcpProvider.CreateFirewallRules(ctx, networkName); err != nil {
		l.Error(fmt.Sprintf("Failed to create firewall rules: %v", err))
		return fmt.Errorf("failed to create firewall rules: %w", err)
	}

	gcpProvider.NetworkName = networkName
	return nil
}

func createResources(
	ctx context.Context,
	gcpProvider *gcp_provider.GCPProvider,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	// Find orchestrator machine first
	var orchestratorMachine models.Machiner
	for _, machine := range m.Deployment.GetMachines() {
		if machine.IsOrchestrator() {
			orchestratorMachine = machine
			break
		}
	}

	if orchestratorMachine == nil {
		return fmt.Errorf("no orchestrator machine found in deployment")
	}

	// Create orchestrator first
	l.Info("Creating orchestrator VM...")
	if err := gcpProvider.CreateAndConfigureVM(ctx, orchestratorMachine); err != nil {
		return fmt.Errorf("failed to create orchestrator VM: %w", err)
	}
	updateMachineConfig(m.Deployment, orchestratorMachine.GetName())

	// Create worker VMs in parallel
	type result struct {
		name string
		err  error
	}
	resultChan := make(chan result)
	activeWorkers := 0

	// Launch worker creation in parallel
	for name, machine := range m.Deployment.GetMachines() {
		if machine.IsOrchestrator() {
			continue
		}
		activeWorkers++
		go func(name string, machine models.Machiner) {
			err := gcpProvider.CreateAndConfigureVM(ctx, machine)
			resultChan <- result{name: name, err: err}
		}(name, machine)
	}

	// Collect results
	failedMachines := []string{}
	for i := 0; i < activeWorkers; i++ {
		res := <-resultChan
		if res.err != nil {
			l.Errorf("Failed to create worker VM %s: %v", res.name, res.err)
			failedMachines = append(failedMachines, res.name)
			delete(m.Deployment.Machines, res.name)
		} else {
			updateMachineConfig(m.Deployment, res.name)
		}
	}

	// Check if we have at least one worker
	if len(m.Deployment.GetMachines()) < 2 { //nolint:mnd
		return fmt.Errorf(
			"deployment requires at least one worker VM, failed machines: %v",
			failedMachines,
		)
	}

	return nil
}

func provisionBacalhauCluster(
	ctx context.Context,
	gcpProvider *gcp_provider.GCPProvider,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	if err := gcpProvider.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}
	for _, machine := range m.Deployment.GetMachines() {
		machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateSucceeded)
		updateMachineConfig(m.Deployment, machine.GetName())
	}
	return nil
}

func finalizeDeployment(ctx context.Context, gcpProvider *gcp_provider.GCPProvider) error {
	if err := gcpProvider.FinalizeDeployment(ctx); err != nil {
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
	machine := deployment.GetMachine(machineName)
	if machine != nil {
		viper.Set(
			fmt.Sprintf(
				"deployments.%s.gcp.%s.%s",
				deployment.UniqueID,
				deployment.GCP.ProjectID,
				machineName,
			),
			models.MachineConfigToWrite(machine),
		)
		writeConfig()
	}
}

func handleDeploymentCompletion(
	ctx context.Context,
	m *display.DisplayModel,
	deploymentErr error,
) {
	writeConfig()

	fmt.Println(m.RenderFinalTable())

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
