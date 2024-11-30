package aws

import (
	"sync"

	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var once sync.Once
var AwsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related commands",
	Long:  `Commands for interacting with AWS resources.`,
}

// GetAwsDiagnosticsCmd returns the diagnostics command
func GetAwsDiagnosticsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Print AWS configuration diagnostics",
		Long:  `Prints detailed information about AWS configuration, credentials, and permissions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			accountID := viper.GetString("aws.account_id")
			awsProvider, err := aws_provider.NewAWSProviderFunc(accountID)
			if err != nil {
				return err
			}

			return awsProvider.PrintDiagnostics(cmd.Context())
		},
	}

	return cmd
}

func GetAwsCmd() *cobra.Command {
	InitializeCommands()
	return AwsCmd
}

func InitializeCommands() {
	once.Do(func() {
		AwsCmd.AddCommand(GetAwsListDeploymentsCmd())
		AwsCmd.AddCommand(GetAwsCreateDeploymentCmd())
		AwsCmd.AddCommand(GetAwsDestroyCmd())
		AwsCmd.AddCommand(GetAwsDiagnosticsCmd())
	})
}
