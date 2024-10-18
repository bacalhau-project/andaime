package aws

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	instanceTypeFlag string
)

//nolint:unused
var CreateDeploymentCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Create a deployment in AWS",
	Long:  `Create a deployment in AWS using the configuration specified in the config file.`,
	RunE:  ExecuteCreateDeployment,
}

func init() {
	CreateDeploymentCmd.Flags().
		StringVar(&instanceTypeFlag, "instance-type", "EC2", "Type of instance to deploy (EC2 or Spot)")
}

//nolint:unused
func ExecuteCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()

	awsProvider, err := aws_provider.NewAWSProvider(viper.GetViper())
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}

	instanceType := awsprovider.EC2Instance // Default to EC2 instance
	if instanceTypeFlag == "Spot" {
		instanceType = awsprovider.SpotInstance
	}
	if err := awsProvider.CreateDeployment(cmd.Context(), instanceType); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	l.Info("AWS deployment created successfully")
	return nil
}
