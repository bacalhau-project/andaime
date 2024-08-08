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

	// Subscriptions API
	NewSubscriptionListPager(
		ctx context.Context,
		options *armsubscription.SubscriptionsClientListOptions,
	) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]

	ListAllResourcesInSubscription(
		ctx context.Context,
		subscriptionID string,
		tags map[string]*string,
	) (AzureResources, error)

	ListTypedResources(
		ctx context.Context,
		subscriptionID string,
		resourceGroupName string,
		tags map[string]*string,
	) (AzureResources, error)

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
	return location + "-nsg"
}

func getSubnetName(location string) string {
	return location + "-subnet"
}

func getNetworkInterfaceName(machineID string) string {
	return machineID + "-nic"
}

func getPublicIPName(machineID string) string {
	return machineID + "-ip"
}
func getVMName(machineID string) string {
	return machineID + "-vm"
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

func (p *AzureProvider) ListTypedResources(ctx context.Context,
	subscriptionID string,
	resourceGroupName string,
	tags map[string]*string) (AzureResources, error) {
	if p.Client == nil {
		return AzureResources{}, fmt.Errorf("AzureProvider.Client is nil")
	}

	searchScope := fmt.Sprintf("resourceGroup eq '%s'", resourceGroupName)
	if len(tags) > 0 {
		tagFilters := make([]string, 0, len(tags))
		for k, v := range tags {
			if v == nil {
				return AzureResources{}, fmt.Errorf("nil value for tag key '%s'", k)
			}
			tagFilters = append(tagFilters, fmt.Sprintf("tags['%s'] eq '%s'", k, *v))
		}
		searchScope += " and " + strings.Join(tagFilters, " and ")
	}

	resourcesResponse, err := p.Client.ListTypedResources(ctx,
		subscriptionID,
		searchScope,
		tags)
	if err != nil {
		logger.Get().Errorf("Failed to query Azure resources: %v", err)
		return AzureResources{}, fmt.Errorf("failed to query resources: %w", err)
	}

	return resourcesResponse, nil
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
	_, err := c.resourceGroupsClient.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *LiveAzureClient) ListAllResourcesInSubscription(
	ctx context.Context,
	subscriptionID string,
	tags map[string]*string) (AzureResources, error) {
	return c.ListTypedResources(ctx, subscriptionID, "", tags)
}

func (c *LiveAzureClient) ListTypedResources(
	ctx context.Context,
	subscriptionID string,
	resourceGroupName string,
	tags map[string]*string) (AzureResources, error) {

	// --- START OF MERGED SearchResources functionality ---

	query := `Resources 
    | project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
        properties, sku, identity, zones, plan, kind, managedBy, 
        provisioningState = tostring(properties.provisioningState)`

	// If searchScope is not the subscriptionID, it's a resource group name
	if resourceGroupName != "" {
		query += fmt.Sprintf(" | where resourceGroup == '%s'", resourceGroupName)
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
	if err != nil {
		return AzureResources{}, fmt.Errorf("failed to query resources: %v", err)
	}

	if res.Data == nil || len(res.Data.([]interface{})) == 0 {
		return AzureResources{}, nil
	}

	// --- END OF MERGED SearchResources functionality ---

	// Process results into typed resources
	var resources AzureResources
	for _, item := range res.Data.([]interface{}) {
		SetTypedResourceFromInterface(&resources, item.(map[string]interface{}))
	}

	return resources, nil
}
