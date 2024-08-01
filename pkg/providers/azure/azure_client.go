//nolint:lll
package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	azureutils "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
)

func IsValidVMSize(vmSize string) bool {
	validSizes := viper.GetStringSlice("azure.valid_vm_sizes")
	for _, size := range validSizes {
		if size == vmSize {
			return true
		}
	}
	return false
}

type AzureClient interface {
	// Resource Group API
	GetOrCreateResourceGroup(
		ctx context.Context,
		location, name string,
		tags map[string]*string,
	) (*armresources.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context, resourceGroupName string) error

	// ResourceGraphClientAPI
	SearchResources(
		ctx context.Context,
		resourceGroup string,
		subscriptionID string,
		tags map[string]*string,
	) (armresourcegraph.ClientResourcesResponse, error)

	// Subscriptions API
	NewSubscriptionListPager(
		ctx context.Context,
		options *armsubscription.SubscriptionsClientListOptions,
	) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]

	// New methods for ARM template deployment
	DeployTemplate(
		ctx context.Context,
		resourceGroupName string,
		deploymentName string,
		template map[string]interface{},
		parameters map[string]interface{},
		tags map[string]*string,
	) (*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse], error)
	GetDeploymentsClient() *armresources.DeploymentsClient
}

// LiveAzureClient wraps all Azure SDK calls
type LiveAzureClient struct {
	resourceGroupsClient *armresources.ResourceGroupsClient
	resourceGraphClient  *armresourcegraph.Client
	subscriptionsClient  *armsubscription.SubscriptionsClient
	deploymentsClient    *armresources.DeploymentsClient
}

func (c *LiveAzureClient) GetDeploymentsClient() *armresources.DeploymentsClient {
	return c.deploymentsClient
}

var NewAzureClientFunc = NewAzureClient

// NewAzureClient creates a new AzureClient
func NewAzureClient(subscriptionID string) (AzureClient, error) {
	// Get credential from CLI
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	// Create Azure clients
	resourceGroupsClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	subscriptionsClient, err := armsubscription.NewSubscriptionsClient(cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	resourceGraphClient, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	deploymentsClient, err := armresources.NewDeploymentsClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}

	return &LiveAzureClient{
		resourceGroupsClient: resourceGroupsClient,
		resourceGraphClient:  resourceGraphClient,
		subscriptionsClient:  subscriptionsClient,
		deploymentsClient:    deploymentsClient,
	}, nil
}

func getVnetName(location string) string {
	return location + "-vnet"
}

func getNetworkSecurityGroupName(location string) string {
	return location + "-" + "-nsg"
}

func getSubnetName(location string) string {
	return location + "-subnet"
}

func getNetworkInterfaceName(machineID string) string {
	return machineID + "-nic"
}

func getPublicIPName(machineID string) string {
	return fmt.Sprintf("%s-ip", machineID)
}
func getVMName(machineID string) string {
	return "vm-" + machineID
}

func (c *LiveAzureClient) DeployTemplate(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
	template map[string]interface{},
	parameters map[string]interface{},
	tags map[string]*string,
) (*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse], error) {
	l := logger.Get()
	l.Debugf("DeployTemplate: Beginning - %s", deploymentName)

	deploymentParams := armresources.Deployment{
		Properties: &armresources.DeploymentProperties{
			Template:   template,
			Parameters: &parameters,
			Mode:       to.Ptr(armresources.DeploymentModeIncremental),
		},
		Tags: tags,
	}

	future, err := c.GetDeploymentsClient().BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		deploymentName,
		deploymentParams,
		nil,
	)
	if err != nil {
		l.Errorf("DeployTemplate: %s", err)
		return nil, err
	}

	return future, nil
}

func IsValidLocation(location string) bool {
	locations, err := azureutils.GetLocations()
	if err != nil {
		logger.Get().Errorf("Failed to get Azure locations: %v", err)
		return false
	}
	for _, validLocation := range locations {
		if strings.EqualFold(location, validLocation) {
			return true
		}
	}
	return false
}

func (c *LiveAzureClient) SearchResources(ctx context.Context,
	resourceGroup string,
	subscriptionID string,
	tags map[string]*string) (armresourcegraph.ClientResourcesResponse, error) {
	logger.LogAzureAPIStart("SearchResources")
	query := "Resources | project id, name, type, location, tags"
	if resourceGroup != "" {
		query += fmt.Sprintf(" | where resourceGroup == '%s'", resourceGroup)
	}
	for key, value := range tags {
		if value != nil {
			query += fmt.Sprintf(" | where tags['%s'] == '%s'", key, *value)
		}
	}

	request := armresourcegraph.QueryRequest{
		Query:         to.Ptr(query),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}

	res, err := c.resourceGraphClient.Resources(ctx, request, nil)
	logger.LogAzureAPIEnd("SearchResources", err)
	if err != nil {
		return armresourcegraph.ClientResourcesResponse{}, fmt.Errorf(
			"failed to query resources: %v",
			err,
		)
	}

	if res.Data == nil || len(res.Data.([]interface{})) == 0 {
		fmt.Println("No resources found")
		return armresourcegraph.ClientResourcesResponse{}, nil
	}

	return res, nil
}

func (c *LiveAzureClient) NewSubscriptionListPager(
	ctx context.Context,
	options *armsubscription.SubscriptionsClientListOptions,
) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	return c.subscriptionsClient.NewListPager(options)
}

func (c *LiveAzureClient) DestroyResourceGroup(
	ctx context.Context,
	resourceGroupName string,
) error {
	logger.LogAzureAPIStart("DestroyResourceGroup")
	_, err := c.resourceGroupsClient.BeginDelete(ctx, resourceGroupName, nil)
	logger.LogAzureAPIEnd("DestroyResourceGroup", err)
	if err != nil {
		return err
	}

	return nil
}
