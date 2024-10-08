package azure

import (
	"errors"
	"net"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/bacalhau-project/andaime/pkg/logger"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
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

		projectPrefix := viper.GetString("general.project_prefix")
		uniqueID := viper.GetString("general.unique_id")
		tags := azure_provider.GenerateTags(projectPrefix, uniqueID)

		log.Info("Listing Azure resources...")

		azureProvider, err := azure_provider.NewAzureProviderFunc(
			cmd.Context(),
			viper.GetString("azure.subscription_id"),
		)
		if err != nil {
			log.Fatal("failed to assert provider to common.AzureProviderer")
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

		var resources []interface{}
		if err != nil {
			log.Fatalf("Failed to get subscription ID: %v", err)
		}
		if allFlag {
			resources, err = azureProvider.
				ListAllResourcesInSubscription(cmd.Context(),
					tags)
		} else {
			resources, err = azureProvider.GetResources(cmd.Context(),
				resourceGroup,
				tags)
		}

		_ = resources // TODO: Figure out if this is still necessary

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

	},
}

//nolint:gochecknoinits
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
