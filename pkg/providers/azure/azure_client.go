//nolint:lll
package azure

import (
	"context"
	"encoding/json"
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
	) ([]armresources.GenericResource, error)

	// Subscriptions API
	NewSubscriptionListPager(
		ctx context.Context,
		options *armsubscription.SubscriptionsClientListOptions,
	) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]

	ListResourcesInGroup(
		ctx context.Context,
		resourceGroupName string,
	) ([]AzureResource, error)

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

func (c *LiveAzureClient) SearchResources(
	ctx context.Context,
	searchScope string,
	subscriptionID string,
	tags map[string]*string) ([]armresources.GenericResource, error) {
	logger.LogAzureAPIStart("SearchResources")
	query := `Resources 
    | project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
        properties, sku, identity, zones, plan, kind, managedBy, 
        provisioningState = tostring(properties.provisioningState)`

	// If searchScope is not the subscriptionID, it's a resource group name
	if searchScope != subscriptionID {
		query += fmt.Sprintf(" | where resourceGroup == '%s'", searchScope)
	} else {
		for key, value := range tags {
			if value != nil {
				query += fmt.Sprintf(" | where tags['%s'] == '%s'", key, *value)
			}
		}
	}
	request := armresourcegraph.QueryRequest{
		Query:         to.Ptr(query),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}

	res, err := c.resourceGraphClient.Resources(ctx, request, nil)
	logger.LogAzureAPIEnd("SearchResources", err)
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %v", err)
	}

	if res.Data == nil || len(res.Data.([]interface{})) == 0 {
		fmt.Println("No resources found")
		return nil, nil
	}

	resources := res.Data.([]interface{})
	typedResources := make([]armresources.GenericResource, 0, len(resources))

	for _, resource := range resources {
		resourceMap := resource.(map[string]interface{})

		// Convert the map to JSON
		jsonData, err := json.Marshal(resourceMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal resource: %v", err)
		}

		// Unmarshal JSON into GenericResource
		var typedResource armresources.GenericResource
		if err := json.Unmarshal(jsonData, &typedResource); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource: %v", err)
		}

		typedResources = append(typedResources, typedResource)
	}

	return typedResources, nil
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

func (c *LiveAzureClient) ListResourcesInGroup(
	ctx context.Context,
	resourceGroupName string,
) ([]AzureResource, error) {
	clientResourcesResponse, err := c.resourceGraphClient.Resources(
		ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("Resources | where resourceGroup = '" + resourceGroupName + "'"),
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	var resources []AzureResource
	for _, item := range clientResourcesResponse.Data.([]interface{}) {
		var resource AzureResource
		data, _ := json.Marshal(item)
		if err := json.Unmarshal(data, &resource); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}

	return resources, nil
}
