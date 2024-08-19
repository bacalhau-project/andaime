//nolint:lll
package azure

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	azureutils "github.com/bacalhau-project/andaime/internal/clouds/azure"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
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

var skippedTypes = []string{
	"microsoft.compute/virtualmachines/extensions",
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

	ListAllResourceGroups(
		ctx context.Context,
	) (map[string]string, error)

	GetResourceGroup(
		ctx context.Context,
		location, name string,
	) (*armresources.ResourceGroup, error)

	ListAllResourcesInSubscription(
		ctx context.Context,
		subscriptionID string,
		tags map[string]*string,
	) ([]interface{}, error)

	GetResources(
		ctx context.Context,
		subscriptionID string,
		resourceGroupName string,
		tags map[string]*string,
	) ([]interface{}, error)

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
	GetVirtualMachine(
		ctx context.Context,
		resourceGroupName string,
		vmName string,
	) (*armcompute.VirtualMachine, error)
	GetPublicIPAddress(
		ctx context.Context,
		resourceGroupName string,
		publicIP *armnetwork.PublicIPAddress,
	) (string, error)
	GetNetworkInterface(
		ctx context.Context,
		resourceGroupName string,
		networkInterfaceName string,
	) (*armnetwork.Interface, error)
}

// LiveAzureClient wraps all Azure SDK calls
type LiveAzureClient struct {
	resourceGroupsClient *armresources.ResourceGroupsClient
	resourceGraphClient  *armresourcegraph.Client
	subscriptionsClient  *armsubscription.SubscriptionsClient
	deploymentsClient    *armresources.DeploymentsClient
	computeClient        *armcompute.VirtualMachinesClient
	networkClient        *armnetwork.InterfacesClient
	publicIPClient       *armnetwork.PublicIPAddressesClient
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
	computeClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	networkClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}

	return &LiveAzureClient{
		resourceGroupsClient: resourceGroupsClient,
		resourceGraphClient:  resourceGraphClient,
		subscriptionsClient:  subscriptionsClient,
		deploymentsClient:    deploymentsClient,
		computeClient:        computeClient,
		networkClient:        networkClient,
		publicIPClient:       publicIPClient,
	}, nil
}

func (c *LiveAzureClient) DeployTemplate(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
	template map[string]interface{},
	params map[string]interface{},
	tags map[string]*string,
) (*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse], error) {
	l := logger.Get()
	l.Debugf("DeployTemplate: Beginning - %s", deploymentName)

	wrappedParams := make(map[string]interface{})
	for k, v := range params {
		wrappedParams[k] = map[string]interface{}{"Value": v}
	}

	paramsMap, err := utils.StructToMap(wrappedParams)
	if err != nil {
		return nil, fmt.Errorf("failed to convert struct to map: %w", err)
	}

	deploymentParams := armresources.Deployment{
		Properties: &armresources.DeploymentProperties{
			Template:   template,
			Parameters: &paramsMap,
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
	fmt.Println("Destroying resource group", resourceGroupName)
	_, err := c.resourceGroupsClient.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *LiveAzureClient) ListAllResourceGroups(
	ctx context.Context,
) (map[string]string, error) {
	rgList := c.resourceGroupsClient.NewListPager(nil)
	resourceGroups := make(map[string]string)
	for rgList.More() {
		page, err := rgList.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, rg := range page.ResourceGroupListResult.Value {
			resourceGroups[*rg.Name] = *rg.Location
		}
	}
	return resourceGroups, nil
}

func (c *LiveAzureClient) ListAllResourcesInSubscription(
	ctx context.Context,
	subscriptionID string,
	tags map[string]*string) ([]interface{}, error) {
	return c.GetResources(ctx, subscriptionID, "", tags)
}

func (c *LiveAzureClient) GetResources(
	ctx context.Context,
	subscriptionID string,
	resourceGroupName string,
	tags map[string]*string) ([]interface{}, error) {
	l := logger.Get()
	prog := display.GetGlobalProgram()
	m := display.GetGlobalModelFunc()
	// Remove the state machine reference
	// --- START OF MERGED SearchResources functionality ---

	query := `Resources 
    | project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
        properties, sku, identity, zones, plan, kind, managedBy, 
        provisioningState = tostring(properties.provisioningState)`

	// If searchScope is not the subscriptionID, it's a resource group name
	if resourceGroupName != "" {
		query += fmt.Sprintf(" | where resourceGroup == '%s'", resourceGroupName)
	}
	request := armresourcegraph.QueryRequest{
		Query:         to.Ptr(query),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}

	res, err := c.resourceGraphClient.Resources(ctx, request, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %v", err)
	}

	if res.Data == nil || len(res.Data.([]interface{})) == 0 {
		l.Debugf("No resources found")
		return []interface{}{}, nil
	}

	l.Debugf("Found %d resources", len(res.Data.([]interface{})))

	for _, resource := range res.Data.([]interface{}) {
		resourceMap := resource.(map[string]interface{})
		if slices.ContainsFunc(skippedTypes, func(t string) bool {
			return strings.EqualFold(resourceMap["type"].(string), t)
		}) {
			continue
		}

		statuses, err := models.ConvertFromRawResourceToStatus(resourceMap, m.Deployment)
		if err != nil {
			l.Errorf("Failed to convert resource to status: %v", err)
			continue
		}

		for _, status := range statuses {
			internalStatus := status
			prog.UpdateStatus(&internalStatus)
		}
	}

	return res.Data.([]interface{}), nil
}

func (c *LiveAzureClient) GetVirtualMachine(
	ctx context.Context,
	resourceGroupName string,
	vmName string,
) (*armcompute.VirtualMachine, error) {
	resp, err := c.computeClient.Get(ctx, resourceGroupName, vmName, nil)
	if err != nil {
		return nil, err
	}
	return &resp.VirtualMachine, nil
}

func (c *LiveAzureClient) GetNetworkInterface(
	ctx context.Context,
	resourceGroupName string,
	networkInterfaceName string,
) (*armnetwork.Interface, error) {
	resp, err := c.networkClient.Get(ctx, resourceGroupName, networkInterfaceName, nil)
	if err != nil {
		return nil, err
	}
	return &resp.Interface, nil
}

func (c *LiveAzureClient) GetPublicIPAddress(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddress *armnetwork.PublicIPAddress,
) (string, error) {
	publicID, err := arm.ParseResourceID(*publicIPAddress.ID)
	if err != nil {
		return "", fmt.Errorf("failed to parse public IP address ID: %w", err)
	}

	publicIPResponse, err := c.publicIPClient.Get(
		ctx,
		publicID.ResourceGroupName,
		publicID.Name,
		&armnetwork.PublicIPAddressesClientGetOptions{Expand: nil},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get public IP address: %w", err)
	}
	return *publicIPResponse.Properties.IPAddress, nil
}
