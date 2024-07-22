package aws

import (
	"fmt"

	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create AWS resources",
	Long:  `Create various AWS resources including deployments.`,
}

func init() {
	createCmd.AddCommand(createDeploymentCmd)
}

var createDeploymentCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Create a deployment in AWS",
	Long:  `Create a deployment in AWS using the configuration specified in the config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		if err != nil {
			return fmt.Errorf("failed to initialize AWS provider: %w", err)
		}

		err = awsProvider.CreateDeployment(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}

		cmd.Println("AWS deployment created successfully")
		return nil
	},
}
