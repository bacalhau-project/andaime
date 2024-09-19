// pkg/providers/azure/client.go
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

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"

	azure_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/azure"
)

var skippedTypes = []string{
	"Microsoft.Network/virtualNetworks",
	"Microsoft.Network/networkSecurityGroups",
	"Microsoft.Network/networkInterfaces",
	"Microsoft.Network/publicIPAddresses",
	"Microsoft.Network/loadBalancers",
	"Microsoft.Network/applicationGateways",
}

// AzureError represents a custom error type for Azure operations.
type AzureError struct {
	Code    string
	Message string
}

// Error returns the error message.
func (e AzureError) Error() string {
	return fmt.Sprintf("AzureError: %s - %s", e.Code, e.Message)
}

// IsNotFound checks if the error indicates that a resource was not found.
func (e AzureError) IsNotFound() bool {
	return e.Code == "ResourceNotFound"
}

// LiveAzureClient implements the AzureClienter interface using the Azure SDK.
type LiveAzureClient struct {
	subscriptionID       string
	cred                 *azidentity.DefaultAzureCredential
	resourceGroupsClient *armresources.ResourceGroupsClient
	resourceGraphClient  *armresourcegraph.Client
	resourcesSKUClient   *armcompute.ResourceSKUsClient
	subscriptionsClient  *armsubscription.SubscriptionsClient
	deploymentsClient    *armresources.DeploymentsClient
	computeClient        *armcompute.VirtualMachinesClient
	networkClient        *armnetwork.InterfacesClient
	publicIPClient       *armnetwork.PublicIPAddressesClient
}

// Ensure LiveAzureClient implements the AzureClienter interface.
var _ azure_interface.AzureClienter = &LiveAzureClient{}

// NewAzureClient creates a new AzureClient.
func NewAzureClient(subscriptionID string) (azure_interface.AzureClienter, error) {
	// Get credential from CLI.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	if subscriptionID == "" {
		return nil, fmt.Errorf("subscriptionID is required")
	}

	// Create Azure clients.
	resourceGroupsClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	subscriptionsClient, err := armsubscription.NewSubscriptionsClient(cred, nil)
	if err != nil {
		return nil, err
	}
	resourceGraphClient, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		return nil, err
	}
	deploymentsClient, err := armresources.NewDeploymentsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	computeClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	networkClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}

	resourcesSKUClient, err := armcompute.NewResourceSKUsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}

	return &LiveAzureClient{
		subscriptionID:       subscriptionID,
		cred:                 cred,
		resourceGroupsClient: resourceGroupsClient,
		resourcesSKUClient:   resourcesSKUClient,
		resourceGraphClient:  resourceGraphClient,
		subscriptionsClient:  subscriptionsClient,
		deploymentsClient:    deploymentsClient,
		computeClient:        computeClient,
		networkClient:        networkClient,
		publicIPClient:       publicIPClient,
	}, nil
}

// Implement AzureClienter interface methods.

// DeployTemplate deploys an ARM template.
func (c *LiveAzureClient) DeployTemplate(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
	template map[string]interface{},
	params map[string]interface{},
	tags map[string]*string,
) (azure_interface.Pollerer, error) {
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

	future, err := c.deploymentsClient.BeginCreateOrUpdate(
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

// NewSubscriptionListPager returns a new subscription list pager.
func (c *LiveAzureClient) NewSubscriptionListPager(
	ctx context.Context,
	options *armsubscription.SubscriptionsClientListOptions,
) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	return c.subscriptionsClient.NewListPager(options)
}

// ListAllResourceGroups lists all resource groups.
func (c *LiveAzureClient) ListAllResourceGroups(
	ctx context.Context,
) ([]*armresources.ResourceGroup, error) {
	rgList := c.resourceGroupsClient.NewListPager(nil)
	var resourceGroups []*armresources.ResourceGroup
	for rgList.More() {
		page, err := rgList.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, rg := range page.ResourceGroupListResult.Value {
			resourceGroups = append(resourceGroups, rg)
		}
	}
	return resourceGroups, nil
}

// ListAllResourcesInSubscription lists all resources in a subscription with given tags.
func (c *LiveAzureClient) ListAllResourcesInSubscription(
	ctx context.Context,
	subscriptionID string,
	tags map[string]*string) ([]interface{}, error) {
	return c.GetResources(ctx, subscriptionID, "", tags)
}

// GetResources retrieves resources based on subscriptionID, resourceGroupName, and tags.
func (c *LiveAzureClient) GetResources(
	ctx context.Context,
	subscriptionID string,
	resourceGroupName string,
	tags map[string]*string) ([]interface{}, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	// Remove the state machine reference
	// --- START OF MERGED SearchResources functionality ---

	query := `Resources 
    | project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
        properties, sku, identity, zones, plan, kind, managedBy, 
        provisioningState = tostring(properties.provisioningState)`

	// If resourceGroupName is not empty, filter by resource group.
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
			m.UpdateStatus(&internalStatus)
		}
	}

	return res.Data.([]interface{}), nil
}

// GetVirtualMachine retrieves a virtual machine.
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

// GetNetworkInterface retrieves a network interface.
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

// GetPublicIPAddress retrieves a public IP address.
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

// GetSKUsByLocation retrieves SKUs available in a specific location.
func (c *LiveAzureClient) GetSKUsByLocation(
	ctx context.Context,
	location string,
) ([]armcompute.ResourceSKU, error) {
	// Create a filter for the specific location.
	filter := fmt.Sprintf("location eq '%s'", location)
	pager := c.resourcesSKUClient.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: &filter,
	})

	var skus []armcompute.ResourceSKU
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, sku := range page.Value {
			skus = append(skus, *sku)
		}
	}
	return skus, nil
}

// ValidateMachineType checks if the specified machine type is valid in the given location.
func (c *LiveAzureClient) ValidateMachineType(
	ctx context.Context,
	location string,
	vmSize string,
) (bool, error) {
	// Create a filter for the specific location and VM size.
	filter := fmt.Sprintf("location eq '%s'", location)
	pager := c.resourcesSKUClient.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: &filter,
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to get next page of resource SKUs: %w", err)
		}

		for _, sku := range page.Value {
			if sku.Name != nil && *sku.Name == vmSize && sku.ResourceType != nil &&
				*sku.ResourceType == "virtualMachines" {
				// Check if the SKU is available in the location.
				if sku.Restrictions != nil {
					for _, restriction := range sku.Restrictions {
						if restriction.Type != nil &&
							*restriction.Type == armcompute.ResourceSKURestrictionsTypeLocation {
							return false, fmt.Errorf(
								"VM size %s is not available in location %s",
								vmSize,
								location,
							)
						}
					}
				}
				// If we found the SKU and it's not restricted, it's valid.
				return true, nil
			}
		}
	}

	return false, fmt.Errorf("VM size %s not found in location %s", vmSize, location)
}

// ResourceGroupExists checks if a resource group exists.
func (c *LiveAzureClient) ResourceGroupExists(
	ctx context.Context,
	resourceGroupName string,
) (bool, error) {
	_, err := c.resourceGroupsClient.Get(ctx, resourceGroupName, nil)
	if err != nil {
		if strings.Contains(err.Error(), "ResourceNotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetVMExternalIP retrieves the external IP of a VM instance.
// Note: Implementation depends on how IPs are managed; this is a placeholder.
func (c *LiveAzureClient) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	resourceGroupName := locationData["resourceGroupName"]
	if resourceGroupName == "" {
		return "", fmt.Errorf("resourceGroupName is not set")
	}
	vm, err := c.GetVirtualMachine(ctx, resourceGroupName, vmName)
	if err != nil {
		return "", err
	}

	// Assuming the VM has at least one network interface.
	if vm.Properties.NetworkProfile == nil ||
		len(vm.Properties.NetworkProfile.NetworkInterfaces) == 0 {
		return "", fmt.Errorf("no network interfaces found for VM %s", vmName)
	}

	nicID := vm.Properties.NetworkProfile.NetworkInterfaces[0].ID
	if nicID == nil {
		return "", fmt.Errorf("network interface ID is nil")
	}

	parsedNIC, err := arm.ParseResourceID(*nicID)
	if err != nil {
		return "", fmt.Errorf("failed to parse NIC ID: %w", err)
	}

	nic, err := c.GetNetworkInterface(ctx, parsedNIC.ResourceGroupName, parsedNIC.Name)
	if err != nil {
		return "", err
	}

	if nic.Properties.IPConfigurations == nil || len(nic.Properties.IPConfigurations) == 0 {
		return "", fmt.Errorf("no IP configurations found for NIC %s", parsedNIC.Name)
	}

	publicIPID := nic.Properties.IPConfigurations[0].Properties.PublicIPAddress.ID
	if publicIPID == nil {
		return "", fmt.Errorf("public IP address ID is nil")
	}

	parsedPublicIP, err := arm.ParseResourceID(*publicIPID)
	if err != nil {
		return "", fmt.Errorf("failed to parse Public IP ID: %w", err)
	}

	publicIP, err := c.GetPublicIPAddress(
		ctx,
		parsedPublicIP.ResourceGroupName,
		&armnetwork.PublicIPAddress{
			ID: publicIPID,
		},
	)
	if err != nil {
		return "", err
	}

	return publicIP, nil
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

func (c *LiveAzureClient) ListResourceGroups(
	ctx context.Context,
) ([]*armresources.ResourceGroup, error) {
	resourceGroupsClient, err := armresources.NewResourceGroupsClient(c.subscriptionID, c.cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource groups client: %w", err)
	}

	pager := resourceGroupsClient.NewListPager(nil)
	var resourceGroups []*armresources.ResourceGroup
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list resource groups: %w", err)
		}
		resourceGroups = append(resourceGroups, page.Value...)
	}

	return resourceGroups, nil
}