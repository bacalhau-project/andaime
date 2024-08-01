package azure

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AzureListResourcesCmd = &cobra.Command{
	Use:   "list-resources",
	Short: "List Azure resources",
	Long:  `List all resources in a subscription or specific resource group.`,
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		log := logger.Get()
		log.SetVerbose(verbose)

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
			switch {
			case isNetworkError(err):
				log.Fatal("Network is down. Please check your internet connection and try again.")
			case isAzureServiceError(err):
				log.Fatalf("Azure service error: %v", err)
			default:
				log.Fatalf("Failed to query resources: %v", err)
			}
		}

		log.Infof("Azure API contacted (took %s)", time.Since(startTime).Round(time.Millisecond))

		if len(resources) == 0 {
			log.Warn("No resources created by Andaime were found")
		} else {
			log.Infof("Found %d resources created by Andaime", len(resources))

			resourceTable := table.NewResourceTable(os.Stdout)
			for _, resource := range resources {
				log.Debugf("Processing resource: %s", *resource.Name)
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
	AzureListResourcesCmd.Flags().Bool("verbose", false, "Enable verbose output")
}

func GetAzureListResourcesCmd() *cobra.Command {
	return AzureListResourcesCmd
}

func isNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isAzureServiceError(err error) bool {
	var azErr *azcore.ResponseError
	return errors.As(err, &azErr)
}
