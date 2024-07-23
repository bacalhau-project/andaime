package azure

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createAzureDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in Azure",
	Long:  `Create a deployment in Azure using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func GetAzureCreateDeploymentCmd() *cobra.Command {
	return createAzureDeploymentCmd
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	// Pulls all settings from Viper config
	err = azureProvider.DeployResources(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to deploy resources: %w", err)
	}

	cmd.Println("Azure deployment created successfully")
	return nil
}
