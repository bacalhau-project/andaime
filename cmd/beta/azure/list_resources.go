package azure

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AzureListResourcesCmd = &cobra.Command{
	Use:   "list-resources",
	Short: "List Azure resources",
	Long:  `List all resources in a subscription.`,
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.Get()

		projectID := viper.GetString("general.project_id")
		uniqueID := viper.GetString("general.unique_id")
		tags := make(map[string]*string)
		azure.GenerateTags(projectID, uniqueID)

		log.Info("Listing Azure resources...")

		azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
		if err != nil {
			log.Fatalf("Failed to create Azure provider: %v", err)
		}

		// See if "deployed" is a string or a map
		deployed := viper.Get("deployed")
		if deployed == nil {
			log.Fatal("No deployed configuration found in viper")
		}

		// List all resource groups in viper config
		log.Debugf(
			"Listing resource groups in viper config: %v",
			viper.GetStringMapString("deployed.azure"),
		)

		// Get the Azure Resource Group Name from "deployed.azure" in viper
		rgName := viper.GetString("deployed.azure.resource_group")
		if rgName == "" {
			log.Fatal("No resource group name found in viper")
		}

		log.Info("Contacting Azure API...")
		startTime := time.Now()

		resourceCount := 0
		for {
			result, err := azureProvider.GetClient().
				SearchResources(cmd.Context(), rgName, getSubscriptionID(), tags)
			if err != nil {
				log.Fatalf("Failed to query resources: %v", err)
			}

			// Type assertion for result.Data
			resources, ok := result.Data.([]interface{})
			if !ok {
				log.Fatalf("Unexpected data type in result")
			}

			for _, resource := range resources {
				resourceMap := resource.(map[string]interface{})
				log.Infof("Found resource %s of type %s in location %s",
					resourceMap["name"], resourceMap["type"], resourceMap["location"])
				resourceCount++
			}

			if result.SkipToken == nil || *result.SkipToken == "" {
				break
			}
		}

		log.Infof("Azure API contacted (took %s)", time.Since(startTime).Round(time.Millisecond))

		if resourceCount == 0 {
			log.Warn("No resources created by Andaime were found")
		} else {
			log.Infof("Found %d resources created by Andaime", resourceCount)
		}
	},
}

func GetAzureListResourcesCmd() *cobra.Command {
	return AzureListResourcesCmd
}

func isCreatedByAndaime(resource *armresources.GenericResourceExpanded) bool {
	// If it has the "andaime" tag, it was created by Andaime
	return resource.Tags != nil && resource.Tags["andaime"] != nil
}
