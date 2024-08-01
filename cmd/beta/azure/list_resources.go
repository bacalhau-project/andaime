package azure

import (
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AzureListResourcesCmd = &cobra.Command{
	Use:   "list-resources",
	Short: "List Azure resources",
	Long:  `List all resources in a subscription or specific resource group.`,
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.Get()

		projectID := viper.GetString("general.project_id")
		uniqueID := viper.GetString("general.unique_id")
		tags := azure.GenerateTags(projectID, uniqueID)

		log.Info("Listing Azure resources...")

		azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
		if err != nil {
			log.Fatalf("Failed to create Azure provider: %v", err)
		}

		allFlag, _ := cmd.Flags().GetBool("all")
		resourceGroup, _ := cmd.Flags().GetString("resource-group")

		if !allFlag && resourceGroup == "" {
			log.Fatal("Either --all or --resource-group must be specified")
		}

		if allFlag && resourceGroup != "" {
			log.Fatal("Cannot use both --all and --resource-group flags simultaneously")
		}

		log.Info("Contacting Azure API...")
		startTime := time.Now()

		var searchScope string
		if allFlag {
			searchScope = getSubscriptionID()
		} else {
			searchScope = resourceGroup
		}

		resources, err := azureProvider.GetClient().
			SearchResources(cmd.Context(), searchScope, getSubscriptionID(), tags)
		if err != nil {
			log.Fatalf("Failed to query resources: %v", err)
		}

		log.Infof("Azure API contacted (took %s)", time.Since(startTime).Round(time.Millisecond))

		if len(resources) == 0 {
			log.Warn("No resources created by Andaime were found")
		} else {
			log.Infof("Found %d resources created by Andaime", len(resources))
			
			resourceTable := table.NewResourceTable()
			for _, resource := range resources {
				resourceTable.AddResource(resource, "Azure")
			}
			resourceTable.Render()
		}
	},
}

func init() {
	AzureListResourcesCmd.Flags().Bool("all", false, "List resources from the entire subscription")
	AzureListResourcesCmd.Flags().
		String("resource-group", "", "List resources from a specific resource group")
}

func GetAzureListResourcesCmd() *cobra.Command {
	return AzureListResourcesCmd
}
