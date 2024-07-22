package aws

import (
	"fmt"

	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List AWS resources",
	Long:  `List various AWS resources including deployments.`,
}

func init() {
	listCmd.AddCommand(listDeploymentsCmd)
	// Add more 'list' subcommands as needed
}

var listDeploymentsCmd = &cobra.Command{
	Use:   "deployments",
	Short: "List deployments in AWS",
	Long:  `List all deployments in AWS using the configuration specified in the config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		if err != nil {
			return fmt.Errorf("failed to initialize AWS provider: %w", err)
		}

		deployments, err := awsProvider.ListDeployments(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		// Print deployments
		for _, deployment := range deployments {
			cmd.Println(deployment)
		}

		return nil
	},
}
