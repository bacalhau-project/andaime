package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	RetryTimeout   = 2 * time.Second
	DefaultSSHPort = 22
)

var createAWSDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in AWS",
	Long:  `Create a deployment in AWS using the configuration specified in the config file.`,
	RunE:  ExecuteCreateDeployment,
}

func GetAwsCreateDeploymentCmd() *cobra.Command {
	return createAWSDeploymentCmd
}

func ExecuteCreateDeployment(cmd *cobra.Command, _ []string) error {
	// Load .env file at the beginning
	if err := godotenv.Load(); err != nil {
		logger.Get().Warn(fmt.Sprintf("Error loading .env file: %v", err))
	}

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

	// Ensure EC2 client is initialized
	if awsProvider.EC2Client == nil {
		ec2Client := ec2.NewFromConfig(*awsProvider.Config)
		awsProvider.SetEC2Client(ec2Client)
	}

	m := display.NewDisplayModel(deployment)
	prog := display.GetGlobalProgramFunc()

	// Add error handling for TTY initialization
	if err := prog.InitProgram(m); err != nil {
		// Log the TTY error but don't fail the deployment
		logger.Get().
			Warn(fmt.Sprintf("Failed to initialize display: %v. Continuing without interactive display.", err))
	}

	go startResourcePolling(ctx, awsProvider)

	deploymentErr := runDeploymentAsync(ctx, awsProvider, cancel)

	// Only cancel if there's an actual deployment error
	if deploymentErr != nil {
		cancel()
	}

	handleDeploymentCompletion(ctx, m, deploymentErr)

	return deploymentErr
}

func initializeAWSProvider() (*awsprovider.AWSProvider, error) {
	// Try environment variables first, then fall back to viper config
	accountID := os.Getenv("AWS_ACCOUNT_ID")
	if accountID == "" {
		accountID = viper.GetString("aws.account_id")
	}
	if accountID == "" {
		return nil, fmt.Errorf(
			"AWS account ID is required. Set AWS_ACCOUNT_ID in .env file or aws.account_id in config",
		)
	}

	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = viper.GetString("aws.region")
	}
	if region == "" {
		return nil, fmt.Errorf(
			"AWS region is required. Set AWS_DEFAULT_REGION in .env file or aws.region in config",
		)
	}

	awsProvider, err := awsprovider.NewAWSProvider(accountID, region)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS provider: %w", err)
	}
	return awsProvider, nil
}

func prepareDeployment(
	ctx context.Context,
	awsProvider *awsprovider.AWSProvider,
) (*models.Deployment, error) {
	deployment, err := awsProvider.PrepareDeployment(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	machines, locations, err := awsProvider.ProcessMachinesConfig(ctx)
	if err != nil {
		if err.Error() == "no machines configuration found for provider aws" {
			fmt.Println("You can check the instance types available in the AWS Console.")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to process machines config: %w", err)
	}
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	return deployment, nil
}

func startResourcePolling(ctx context.Context, awsProvider *awsprovider.AWSProvider) {
	l := logger.Get()
	err := awsProvider.StartResourcePolling(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to start resource polling: %v", err))
	}
}

func runDeploymentAsync(
	ctx context.Context,
	awsProvider *awsprovider.AWSProvider,
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

	// Only try to run the display if we have a program
	if prog != nil {
		_, err := prog.Run()
		if err != nil {
			// Log the error but don't fail the deployment
			l.Warn(fmt.Sprintf("Display error: %v. Continuing without interactive display.", err))
		}
	}

	select {
	case <-deploymentDone:
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			l.Debug("Context cancelled, waiting for deployment to finish")
			<-deploymentDone
		}
	}

	return deploymentErr
}

func runDeployment(ctx context.Context, awsProvider *awsprovider.AWSProvider) error {
	l := logger.Get()
	prog := display.GetGlobalProgramFunc()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	if err := awsProvider.BootstrapEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap environment: %w", err)
	}

	// Create infrastructure and wait for it to be ready
	if err := awsProvider.CreateInfrastructure(ctx); err != nil {
		return fmt.Errorf("failed to create infrastructure: %w", err)
	}

	// Wait for network propagation and connectivity
	l.Info("Waiting for network propagation...")
	if err := awsProvider.WaitForNetworkConnectivity(ctx); err != nil {
		return fmt.Errorf("failed waiting for network connectivity: %w", err)
	}

	l.Info("Network connectivity confirmed")

	// Wait for all VMs to be accessible via SSH
	l.Info("Waiting for all VMs to be accessible via SSH...")
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	for _, machine := range m.Deployment.GetMachines() {
		if machine.GetPublicIP() == "" {
			return fmt.Errorf("machine %s has no public IP", machine.GetName())
		}

		sshConfig, err := sshutils.NewSSHConfigFunc(
			machine.GetPublicIP(),
			machine.GetSSHPort(),
			machine.GetSSHUser(),
			machine.GetSSHPrivateKeyPath(),
		)
		if err != nil {
			return fmt.Errorf("failed to create SSH config for machine %s: %w", machine.GetName(), err)
		}

		if err := sshConfig.WaitForSSH(ctx, sshutils.SSHRetryAttempts, sshutils.GetAggregateSSHTimeout()); err != nil {
			return fmt.Errorf("failed to establish SSH connection to machine %s: %w", machine.GetName(), err)
		}

		l.Infof("Machine %s is accessible via SSH", machine.GetName())
	}

	l.Info("All VMs are accessible via SSH")

	// Now provision the Bacalhau cluster
	if err := awsProvider.ProvisionBacalhauCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision Bacalhau cluster: %w", err)
	}

	if err := awsProvider.FinalizeDeployment(ctx); err != nil {
		return fmt.Errorf("failed to finalize deployment: %w", err)
	}

	l.Info("Deployment finalized")
	time.Sleep(RetryTimeout)
	prog.Quit()

	return nil
}

func handleDeploymentCompletion(ctx context.Context, m *display.DisplayModel, deploymentErr error) {
	writeConfig()

	// Only try to render the final table if we have a display model
	if m != nil {
		fmt.Println(m.RenderFinalTable())
	}

	if deploymentErr != nil {
		fmt.Println("Deployment failed, but configuration was written to file.")
		fmt.Println("The deployment error was:")
		fmt.Println(deploymentErr)
	}

	if ctx.Err() != nil && ctx.Err() != context.Canceled {
		fmt.Println("General (unknown) error running program:")
		fmt.Println(ctx.Err())
	}
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
