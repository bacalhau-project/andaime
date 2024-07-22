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

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	config := viper.GetViper()
	if config == nil {
		return fmt.Errorf("failed to get configuration")
	}

	azureProvider, err := azure.AzureProviderFunc(config)
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	// Perform deployment
	err = azureProvider.CreateDeployment(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	cmd.Println("Azure deployment created successfully")
	return nil
}
