package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/spf13/cobra"
)

var AzureListResourcesCmd = &cobra.Command{
	Use:   "list-resources",
	Short: "List Azure resources",
	Long:  `List all resources in a subscription.`,
	Run: func(cmd *cobra.Command, args []string) {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Fatalf("failed to obtain a credential: %v", err)
		}
		subID := getSubscriptionID()

		client, err := armresources.NewClient(subID, cred, nil)
		if err != nil {
			log.Fatalf("failed to create a client: %v", err)
		}

		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(context.Background())
			if err != nil {
				log.Fatalf("failed to advance page: %v", err)
			}
			for _, resource := range page.Value {
				if isCreatedByAndaime(resource) {
					fmt.Printf("Resource: %s, Type: %s, Location: %s\n", *resource.Name, *resource.Type, *resource.Location)
				}
			}
		}
	},
}

func isCreatedByAndaime(resource *armresources.GenericResourceExpanded) bool {
	// If it has the "andaime" tag, it was created by Andaime
	return resource.Tags != nil && resource.Tags["andaime"] != nil
}
