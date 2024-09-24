package aws

import (
	"fmt"

	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy AWS resources",
	Long:  `Destroy various AWS resources including deployments.`,
}

func SetupDestroyDeploymentCommand(rootCmd *cobra.Command) {
	destroyCmd.AddCommand(destroyDeploymentCmd)
	// Add more 'destroy' subcommands as needed
}

var destroyDeploymentCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Destroy a deployment in AWS",
	Long:  `Destroy a deployment in AWS using the configuration specified in the config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		if err != nil {
			return fmt.Errorf("failed to initialize AWS provider: %w", err)
		}

		err = awsProvider.DestroyDeployment(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to destroy deployment: %w", err)
		}

		// Remove the deployment from the config file
		deploymentID := viper.GetString("aws.deployment_id")
		if deploymentID != "" {
			viper.Set("deployments."+deploymentID, nil)
			err = viper.WriteConfig()
			if err != nil {
				return fmt.Errorf("failed to update config file: %w", err)
			}
		}

		cmd.Println("AWS deployment destroyed successfully and removed from config")
		return nil
	},
}
