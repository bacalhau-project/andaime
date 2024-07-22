package azure

import (
	"context"
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

		log.Info("Listing Azure resources...")

		azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
		if err != nil {
			log.Fatalf("Failed to create Azure provider: %v", err)
		}

		log.Info("Contacting Azure API...")
		startTime := time.Now()
		pager, err := azureProvider.GetClient().NewListPager(cmd.Context(), "", nil, "")
		if err != nil {
			log.Fatalf("Failed to get resources client: %v", err)
		}
		log.Infof("Azure API contacted (took %s)", time.Since(startTime).Round(time.Millisecond))

		resourceCount := 0
		log.Info("Fetching resources...")
		startTime = time.Now()
		for pager.More() {
			page, err := pager.NextPage(context.Background())
			if err != nil {
				log.Fatalf("Failed to advance page: %v", err)
			}
			for _, resource := range page.Value {
				if isCreatedByAndaime(resource) {
					log.Infof("Found resource %s of type %s in location %s", *resource.Name, *resource.Type, *resource.Location)
					resourceCount++
				}
			}
		}
		log.Infof("Finished fetching resources (took %s)", time.Since(startTime).Round(time.Millisecond))

		if resourceCount == 0 {
			log.Warn("No resources created by Andaime were found")
		} else {
			log.Infof("Found %d resources created by Andaime", resourceCount)
		}
	},
}

func isCreatedByAndaime(resource *armresources.GenericResourceExpanded) bool {
	// If it has the "andaime" tag, it was created by Andaime
	return resource.Tags != nil && resource.Tags["andaime"] != nil
}
