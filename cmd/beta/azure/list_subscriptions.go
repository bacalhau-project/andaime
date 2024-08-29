package azure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

// Subscription represents an Azure subscription
type Subscription struct {
	DisplayName    string
	ID             string
	State          string
	SubscriptionID string
	TenantID       string
}

// SubscriptionTable defines the structure for displaying subscription data
type SubscriptionTable struct {
	Title    string
	Columns  []display.DisplayColumn
	LogFile  string
	DataType interface{}
}

// AzureListSubscriptionsCmd represents the command to list Azure subscriptions
var AzureListSubscriptionsCmd = &cobra.Command{
	Use:   "list-subscriptions",
	Short: "List Azure subscriptions",
	Long:  `List all subscriptions and optionally select one to use.`,
	Run:   runListSubscriptions,
}

func GetAzureListSubscriptionsCmd() *cobra.Command {
	cmd := AzureListSubscriptionsCmd
	cmd.Flags().BoolVar(&setSubscription, "set",
		false, "Set the selected subscription in the config file")
	return cmd
}

var setSubscription bool

func runListSubscriptions(cmd *cobra.Command, args []string) {
	l := logger.Get()
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		l.Errorf("Error: Config file not found")
		return
	}

	err := ListSubscriptions(configFile, setSubscription)
	if err != nil {
		l.Errorf("Error: %v", err)
	}
}

func ListSubscriptions(configFilePath string, setSubscription bool) error {
	l := logger.Get()
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		l.Errorf("failed to obtain a credential: %v", err)
		return err
	}

	client, err := armsubscription.NewSubscriptionsClient(cred, nil)
	if err != nil {
		l.Errorf("failed to create client: %v", err)
		return err
	}

	pager := client.NewListPager(nil)
	subscriptions := make([]*armsubscription.Subscription, 0)

	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			l.Errorf("failed to advance page: %v", err)
			return err
		}
		subscriptions = append(subscriptions, page.Value...)
	}

	if len(subscriptions) == 0 {
		l.Errorf("no subscriptions found for this account")
		return fmt.Errorf("no subscriptions found for this account")
	}

	table := createSubscriptionTable(subscriptions)
	table.Render()

	if setSubscription {
		chosenIndex, err := getUserChoice(len(subscriptions))
		if err != nil {
			return err
		}

		chosenSubscription := subscriptions[chosenIndex]
		err = writeSubscriptionToConfig(*chosenSubscription.ID)
		if err != nil {
			l.Errorf("failed to write subscription to config: %v", err)
			return err
		}

		l.Debugf(
			"Subscription '%s' has been set in the config file.",
			*chosenSubscription.DisplayName,
		)
	} else {
		l.Infof("\nTo set a subscription, run this command with the --set flag:")
		l.Infof("andaime azure list-subscriptions --set")
	}

	return nil
}

func createSubscriptionTable(subscriptions []*armsubscription.Subscription) *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Number", "Name", "ID"})

	for i, sub := range subscriptions {
		table.Append([]string{
			strconv.Itoa(i + 1),
			*sub.DisplayName,
			*sub.ID,
		})
	}

	return table
}

func getUserChoice(max int) (int, error) {
	l := logger.Get()
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter the number of the subscription you want to use: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			l.Errorf("failed to read input: %v", err)
			return 0, err
		}
		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > max {
			l.Errorf("Invalid input. Please enter a number between 1 and %d.", max)
			fmt.Printf("Invalid input. Please enter a number between 1 and %d.\n", max)
			continue
		}
		return choice - 1, nil
	}
}

func writeSubscriptionToConfig(subscriptionID string) error {
	l := logger.Get()
	// Extract the UUID from the full subscription ID
	uuidOnly := extractUUID(subscriptionID)

	// Set the new subscription ID in Viper
	viper.Set("azure.subscription_id", uuidOnly)

	// Write the updated configuration back to the file
	err := viper.WriteConfig()
	if err != nil {
		l.Errorf("failed to write updated config: %v", err)
		return err
	}

	return nil
}

// Helper function to extract UUID from the full subscription ID
func extractUUID(subscriptionID string) string {
	parts := strings.Split(subscriptionID, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return subscriptionID // Return original if splitting fails
}
