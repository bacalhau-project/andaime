package azure

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

var once sync.Once

var AzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Azure-related commands",
	Long:  `Commands for interacting with Azure resources.`,
}

func GetAzureCmd() *cobra.Command {
	InitializeCommands()
	return AzureCmd
}

func InitializeCommands() {
	once.Do(func() {
		AzureCmd.AddCommand(GetAzureListSubscriptionsCmd())
		AzureCmd.AddCommand(GetAzureListResourcesCmd())
		AzureCmd.AddCommand(GetAzureCreateDeploymentCmd())
		AzureCmd.AddCommand(GetAzureListSKUsCmd())
		AzureCmd.AddCommand(GetAzureDestroyCmd())
	})
}

func getSubscriptionID() string {
	log := logger.Get()

	azureCfg := viper.Sub("azure")
	if azureCfg == nil {
		log.Debug("Azure config is nil")
		fmt.Fprintf(os.Stderr, "Error: Azure configuration not found in config file.\n")
		fmt.Fprintf(
			os.Stderr,
			"Please ensure your config file includes an 'azure' section with a 'subscription_id'.\n",
		)
		os.Exit(1)
	}

	subID := azureCfg.GetString("subscription_id")
	if subID == "" {
		subID = viper.GetString("AZURE_SUBSCRIPTION_ID")
	}

	if subID == "" {
		fmt.Fprintf(os.Stderr, "Error: No Azure subscription ID found.\n")
		fmt.Fprintf(
			os.Stderr,
			`Please set it in your config file under azure.subscription_id or as an 
AZURE_SUBSCRIPTION_ID environment variable.`,
		)
		os.Exit(1)
	}

	return subID
}
