package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	RetryTimeout   = 2 * time.Second
	DefaultSSHPort = 22
)

var (
	instanceTypeFlag string
)

var createAWSDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in AWS",
	Long:  `Create a deployment in AWS using the configuration specified in the config file.`,
	RunE:  ExecuteCreateDeployment,
}

func GetAwsCreateDeploymentCmd() *cobra.Command {
	createAWSDeploymentCmd.Flags().
		StringVar(&instanceTypeFlag, "instance-type", "EC2", "Type of instance to deploy (EC2 or Spot)")
	return createAWSDeploymentCmd
}

func ExecuteCreateDeployment(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	awsProvider, err := initializeAWSProvider()
	if err != nil {
		return err
	}

	deployment, err := prepareDeployment(ctx, awsProvider)
	if err != nil {
		return err
	}

	m := display.NewDisplayModel(deployment)
	prog := display.GetGlobalProgramFunc()
	prog.InitProgram(m)

	go startResourcePolling(ctx, awsProvider)

	deploymentErr := runDeploymentAsync(ctx, awsProvider, cancel)

	handleDeploymentCompletion(ctx, m, deploymentErr)

	return nil
}

func initializeAWSProvider() (*aws_provider.AWSProvider, error) {
	awsProvider, err := aws_provider.NewAWSProvider(viper.GetViper())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS provider: %w", err)
	}
	return awsProvider.(*aws_provider.AWSProvider), nil
}

func prepareDeployment(
	ctx context.Context,
	awsProvider *aws_provider.AWSProvider,
) (*models.Deployment, error) {
	deployment, err := awsProvider.PrepareDeployment(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	machines, locations, err := awsProvider.ProcessMachinesConfig(ctx)
	if err != nil {
		if err.Error() == "no machines configuration found for provider aws" {
			fmt.Println(
				"You can check the instance types available with the AWS CLI or Console.",
			)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to process machines config: %w", err)
	}
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	return deployment, nil
}

func startResourcePolling(ctx context.Context, awsProvider *aws_provider.AWSProvider) {
	l := logger.Get()
	err := awsProvider.StartResourcePolling(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to start resource polling: %v", err))
	}
}

func runDeploymentAsync(
	ctx context.Context,
	awsProvider *aws_provider.AWSProvider,
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
			deploymentErr = runDeployment(ctx, awsProvider)
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

func runDeployment(ctx context.Context, awsProvider *aws_provider.AWSProvider) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	// Create VPC and Subnet
	if err := createVPCAndSubnet(ctx, awsProvider); err != nil {
		return err
	}

	// Create resources
	if err := createResources(ctx, awsProvider); err != nil {
		return err
	}

	// Provision Bacalhau cluster
	if err := provisionBacalhauCluster(ctx, awsProvider); err != nil {
		return err
	}

	// Finalize deployment
	if err := finalizeDeployment(ctx, awsProvider); err != nil {
		return err
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}

func createVPCAndSubnet(
	ctx context.Context,
	awsProvider *aws_provider.AWSProvider,
) error {
	if awsProvider.VPCID == "" || awsProvider.SubnetID == "" {
		return fmt.Errorf("VPC ID or Subnet ID is not set")
	}

	if err := awsProvider.CreateVPCAndSubnet(ctx); err != nil {
		return fmt.Errorf("failed to create VPC and subnet: %w", err)
	}
	return nil
}

func createResources(
	ctx context.Context,
	awsProvider *aws_provider.AWSProvider,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	if err := awsProvider.CreateDeployment(ctx); err != nil {
		return fmt.Errorf("failed to create resources: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		updateMachineConfig(m.Deployment, machine.GetName())
	}
	return nil
}

func provisionBacalhauCluster(
	ctx context.Context,
	awsProvider *aws_provider.AWSProvider,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	if err := awsProvider.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		updateMachineConfig(m.Deployment, machine.GetName())
	}
	return nil
}

func finalizeDeployment(ctx context.Context, awsProvider *aws_provider.AWSProvider) error {
	if err := awsProvider.FinalizeDeployment(ctx); err != nil {
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
				"deployments.%s.aws.%s",
				deployment.UniqueID,
				machineName,
			),
			models.MachineConfigToWrite(machine),
		)
		writeConfig()
	}
}

func handleDeploymentCompletion(ctx context.Context, m *display.DisplayModel, deploymentErr error) {
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
