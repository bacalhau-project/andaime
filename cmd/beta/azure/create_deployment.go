package azure

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
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
	logger.InitProduction(false, true)
	log := logger.Get()

	azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
	if err != nil {
		errString := fmt.Sprintf("Failed to initialize Azure provider: %s", err.Error())
		log.Error(errString)
		return fmt.Errorf(errString)
	}
	disp := display.NewDisplay(1000)
	disp.Start(nil)

	// Pulls all settings from Viper config
	err = azureProvider.DeployResources(cmd.Context())
	if err != nil {
		errString := fmt.Sprintf("Failed to deploy resources: %s", err.Error())
		log.Error(errString)
		return fmt.Errorf(errString)
	}

	log.Info("Azure deployment created successfully")
	cmd.Println("Azure deployment created successfully")
	return nil
}
