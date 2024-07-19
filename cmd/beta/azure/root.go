package azure

import (
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/logger"
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
		AzureCmd.AddCommand(AzureListSubscriptionsCmd)
		AzureCmd.AddCommand(AzureListResourcesCmd)
	})
}

func getSubscriptionID() string {
	log := logger.Get()
	v := viper.GetViper()
	azureCfg := v.Sub("azure")

	subID := azureCfg.GetString("subscription_id")
	if subID == "" {
		// Try to load it from AZURE_SUBSCRIPTION_ID
		subID = v.GetString("AZURE_SUBSCRIPTION_ID")
	}

	if subID == "" {
		log.Fatalf("No Azure subscription ID found in either %s or AZURE_SUBSCRIPTION_ID.", v.ConfigFileUsed())
	}

	return subID
}
