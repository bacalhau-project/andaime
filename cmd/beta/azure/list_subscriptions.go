package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/bacalhau-project/andaime/utils"
	"github.com/spf13/cobra"
)

var AzureListSubscriptionsCmd = &cobra.Command{
	Use:   "list-subscriptions",
	Short: "List Azure subscriptions",
	Long:  `List all subscriptions.`,
	Run: func(cmd *cobra.Command, args []string) {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Fatalf("failed to obtain a credential: %v", err)
		}
		client, err := armsubscriptions.NewClient(cred, nil)
		if err != nil {
			log.Fatalf("failed to create a client: %v", err)
		}

		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(context.Background())
			if err != nil {
				log.Fatalf("failed to advance page: %v", err)
			}
			// Print table header
			fmt.Println("DisplayName\tID\t\t\t\t\tState\tSubscriptionID\t\t\t\tTenantID")
			fmt.Println("---------------------------------------------------------------------------------------------------")

			for _, subscription := range page.Value {
				subState := "--"
				if subscription.State != nil {
					subState = string(*subscription.State)
				}

				fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
					utils.SafeDeref(subscription.DisplayName),
					utils.SafeDeref(subscription.ID),
					subState,
					utils.SafeDeref(subscription.SubscriptionID),
					utils.SafeDeref(subscription.TenantID),
				)
			}
		}
	},
}
