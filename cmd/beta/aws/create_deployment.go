package aws

import (
	"context"
	"fmt"
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
	createAWSDeploymentCmd.Flags().String("config", "", "Path to the configuration file")
	return createAWSDeploymentCmd
}

func ExecuteCreateDeployment(cmd *cobra.Command, _ []string) error {
	l := logger.Get()
	// Load .env file at the beginning
	if err := godotenv.Load(); err != nil {
		logger.Get().Warn(fmt.Sprintf("Error loading .env file: %v", err))
	}

	// Ensure we're using the config file specified in the command
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("failed to get config flag: %w", err)
	}
	viper.SetConfigFile(configFile)

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

	m.Deployment.AWS.Region = awsProvider.Region
	m.Deployment.AWS.AccountID = awsProvider.AccountID

	// Add error handling for TTY initialization
	if err := prog.InitProgram(m); err != nil {
		// Log the TTY error but don't fail the deployment
		logger.Get().
			Warn(fmt.Sprintf("Failed to initialize display: %v. Continuing without interactive display.", err))
	}

	if err := startResourcePolling(ctx, awsProvider); err != nil {
		l.Warn(fmt.Sprintf("Failed to start resource polling: %v", err))
	}

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
	accountID := viper.GetString("aws.account_id")
	if accountID == "" {
		return nil, fmt.Errorf(
			"AWS account ID is required. Set aws.account_id in config",
		)
	}

	region := viper.GetString("aws.region")
	if region == "" {
		return nil, fmt.Errorf(
			"AWS region is required. Set aws.region in config",
		)
	}

	awsProvider, err := awsprovider.NewAWSProviderFunc(accountID, region)
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

	l.Debug("Starting infrastructure creation...")
	// Create infrastructure and wait for it to be ready
	// Create infrastructure and update display
	if err := awsProvider.CreateInfrastructure(ctx); err != nil {
		l.Debugf("Infrastructure creation failed: %v", err)
		for _, machine := range m.Deployment.GetMachines() {
			m.QueueUpdate(display.UpdateAction{
				MachineName: machine.GetName(),
				UpdateData: display.UpdateData{
					UpdateType:    display.UpdateTypeResource,
					ResourceType:  "Infrastructure",
					ResourceState: models.ResourceStateFailed,
				},
			})
		}
		return fmt.Errorf("failed to create infrastructure: %w", err)
	}

	// Wait for network propagation and connectivity
	l.Info("Waiting for network propagation...")
	if err := awsProvider.WaitForNetworkConnectivity(ctx); err != nil {
		return fmt.Errorf("failed waiting for network connectivity: %w", err)
	}

	l.Info("Network connectivity confirmed")

	if err := awsProvider.DeployVMsInParallel(ctx); err != nil {
		return fmt.Errorf("failed to deploy VMs in parallel: %w", err)
	}

	// Wait for all VMs to be accessible via SSH
	l.Info("Waiting for all VMs to be accessible via SSH...")
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
			return fmt.Errorf(
				"failed to create SSH config for machine %s: %w",
				machine.GetName(),
				err,
			)
		}

		if err := sshConfig.WaitForSSH(ctx, sshutils.SSHRetryAttempts, sshutils.GetAggregateSSHTimeout()); err != nil {
			return fmt.Errorf(
				"failed to establish SSH connection to machine %s: %w",
				machine.GetName(),
				err,
			)
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
		// Ensure the deployments section exists
		if !viper.IsSet("deployments") {
			viper.Set("deployments", make(map[string]interface{}))
		}

		// Get the current deployment info
		m := display.GetGlobalModelFunc()
		if m != nil && m.Deployment != nil {
			deploymentID := m.Deployment.UniqueID
			deploymentPath := fmt.Sprintf("deployments.%s", deploymentID)

			// Save minimal deployment details
			machines := make(map[string]interface{})
			for name, machine := range m.Deployment.GetMachines() {
				machines[name] = map[string]interface{}{
					"public_ip":  machine.GetPublicIP(),
					"private_ip": machine.GetPrivateIP(),
					"location":   machine.GetLocation(),
				}
			}

			viper.Set(deploymentPath, map[string]interface{}{
				"provider": "aws",
				"aws": map[string]interface{}{
					"region":     m.Deployment.AWS.Region,
					"account_id": m.Deployment.AWS.AccountID,
					"vpc_id":     m.Deployment.AWS.VPCID,
				},
				"machines": machines,
			})
		}

		if err := viper.WriteConfig(); err != nil {
			l.Error(fmt.Sprintf("Failed to write configuration to file: %v", err))
		} else {
			l.Info(fmt.Sprintf("Configuration written to %s", configFile))
		}
	} else {
		l.Error("No config file specified")
	}
}
