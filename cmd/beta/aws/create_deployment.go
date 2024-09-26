package aws

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

//nolint:unused
var CreateDeploymentCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Create a deployment in AWS",
	Long:  `Create a deployment in AWS using the configuration specified in the config file.`,
	RunE:  ExecuteCreateDeployment,
}

//nolint:unused
func ExecuteCreateDeployment(cmd *cobra.Command, args []string) error {
	l := logger.Get()

	awsProvider, err := aws_provider.NewAWSProvider(viper.GetViper())
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}

	if err := awsProvider.CreateDeployment(cmd.Context()); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	l.Info("AWS deployment created successfully")
	return nil
}
