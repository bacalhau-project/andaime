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

func init() {
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

		cmd.Println("AWS deployment destroyed successfully")
		return nil
	},
}
