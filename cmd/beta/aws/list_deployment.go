package aws

import (
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetAwsListDeploymentsCmd() *cobra.Command {
	return listDeploymentsCmd
}

var listDeploymentsCmd = &cobra.Command{
	Use:   "deployments",
	Short: "List deployments in AWS",
	Long:  `List all deployments in AWS across all regions using the configuration specified in the config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try to load AWS configuration from environment or config file
		accountID := viper.GetString("aws.account_id")

		// If account ID is not set, try to get it from STS
		if accountID == "" {
			cfg, err := awsconfig.LoadDefaultConfig(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to load AWS configuration: %w", err)
			}
			stsClient := sts.NewFromConfig(cfg)
			callerIdentity, err := stsClient.GetCallerIdentity(
				cmd.Context(),
				&sts.GetCallerIdentityInput{},
			)
			if err != nil {
				return fmt.Errorf("failed to get AWS account ID: %w", err)
			}
			accountID = *callerIdentity.Account
		}

		awsProvider, err := aws_provider.NewAWSProviderFunc(accountID)
		if err != nil {
			return fmt.Errorf("failed to initialize AWS provider: %w", err)
		}

		deployments, err := awsProvider.ListDeployments(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		// Print deployments
		if len(deployments) == 0 {
			cmd.Println("No deployments found")
			return nil
		}

		for _, deployment := range deployments {
			cmd.Println(deployment)
		}

		return nil
	},
}
