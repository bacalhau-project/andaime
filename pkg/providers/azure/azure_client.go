//nolint:lll
package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

type AzureClient interface {
	GetLogger() *logger.Logger
	SetLogger(logger *logger.Logger)

	GetOrCreateResourceGroup(ctx context.Context, location string) (*armresources.ResourceGroup, error)
	CreateVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error)
	GetVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string) (armnetwork.VirtualNetwork, error)
	CreatePublicIP(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error)
	GetPublicIP(ctx context.Context, resourceGroupName, ipName string) (armnetwork.PublicIPAddress, error)
	CreateVirtualMachine(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error)
	GetVirtualMachine(ctx context.Context, resourceGroupName, vmName string) (armcompute.VirtualMachine, error)
	CreateNetworkInterface(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error)
	GetNetworkInterface(ctx context.Context, resourceGroupName, nicName string) (armnetwork.Interface, error)
	CreateNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error)
	GetNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string) (armnetwork.SecurityGroup, error)

	// ResourceGraphClientAPI
	NewListPager(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (*runtime.Pager[armresources.ClientListResponse], error)
	SearchResources(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (armresourcegraph.ClientResourcesResponse, error)
}

// LiveAzureClient wraps all Azure SDK calls
type LiveAzureClient struct {
	Logger *logger.Logger

	virtualNetworksClient   *armnetwork.VirtualNetworksClient
	publicIPAddressesClient *armnetwork.PublicIPAddressesClient
	interfacesClient        *armnetwork.InterfacesClient
	virtualMachinesClient   *armcompute.VirtualMachinesClient
	securityGroupsClient    *armnetwork.SecurityGroupsClient
	resourceGraphClient     *armresourcegraph.Client
	resourcesClient         *armresources.Client
	resourceGroupsClient    *armresources.ResourceGroupsClient
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
	virtualNetworksClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	publicIPAddressesClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	interfacesClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	virtualMachinesClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	securityGroupsClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	resourceGraphClient, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}

	resourcesClient, err := armresources.NewClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}

	resourceGroupsClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}

	return &LiveAzureClient{
		Logger: logger.Get(),

		virtualNetworksClient:   virtualNetworksClient,
		publicIPAddressesClient: publicIPAddressesClient,
		interfacesClient:        interfacesClient,
		virtualMachinesClient:   virtualMachinesClient,
		securityGroupsClient:    securityGroupsClient,
		resourceGraphClient:     resourceGraphClient,
		resourcesClient:         resourcesClient,
		resourceGroupsClient:    resourceGroupsClient,
	}, nil
}

func (c *LiveAzureClient) GetLogger() *logger.Logger {
	return c.Logger
}

func (c *LiveAzureClient) SetLogger(logger *logger.Logger) {
	c.Logger = logger
}

// CreateVirtualNetwork creates a new virtual network
func (c *LiveAzureClient) CreateVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
	c.Logger.Infof("CreateVirtualNetwork")
	poller, err := c.virtualNetworksClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreateVirtualNetwork", err)
		return armnetwork.VirtualNetwork{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreateVirtualNetwork", err)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	return resp.VirtualNetwork, nil
}

// GetVirtualNetwork retrieves a virtual network
func (c *LiveAzureClient) GetVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string) (armnetwork.VirtualNetwork, error) {
	logger.LogAzureAPIStart("GetVirtualNetwork")
	resp, err := c.virtualNetworksClient.Get(ctx, resourceGroupName, vnetName, nil)
	logger.LogAzureAPIEnd("GetVirtualNetwork", err)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	return resp.VirtualNetwork, nil
}

// CreatePublicIP creates a new public IP address
func (c *LiveAzureClient) CreatePublicIP(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
	logger.LogAzureAPIStart("CreatePublicIP")
	poller, err := c.publicIPAddressesClient.BeginCreateOrUpdate(ctx, resourceGroupName, ipName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreatePublicIP", err)
		return armnetwork.PublicIPAddress{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreatePublicIP", err)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}

	return resp.PublicIPAddress, nil
}

func (c *LiveAzureClient) GetPublicIP(ctx context.Context, resourceGroupName, ipName string) (armnetwork.PublicIPAddress, error) {
	logger.LogAzureAPIStart("GetPublicIP")
	resp, err := c.publicIPAddressesClient.Get(ctx, resourceGroupName, ipName, nil)
	logger.LogAzureAPIEnd("GetPublicIP", err)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}

	return resp.PublicIPAddress, nil
}

func (c *LiveAzureClient) CreateVirtualMachine(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
	logger.LogAzureAPIStart("CreateVirtualMachine")
	poller, err := c.virtualMachinesClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreateVirtualMachine", err)
		return armcompute.VirtualMachine{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreateVirtualMachine", err)
	if err != nil {
		return armcompute.VirtualMachine{}, err
	}

	return resp.VirtualMachine, nil
}

func (c *LiveAzureClient) GetVirtualMachine(ctx context.Context, resourceGroupName, vmName string) (armcompute.VirtualMachine, error) {
	logger.LogAzureAPIStart("GetVirtualMachine")
	resp, err := c.virtualMachinesClient.Get(ctx, resourceGroupName, vmName, nil)
	logger.LogAzureAPIEnd("GetVirtualMachine", err)
	if err != nil {
		return armcompute.VirtualMachine{}, err
	}

	return resp.VirtualMachine, nil
}

func (c *LiveAzureClient) CreateNetworkInterface(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
	logger.LogAzureAPIStart("CreateNetworkInterface")
	poller, err := c.interfacesClient.BeginCreateOrUpdate(ctx, resourceGroupName, nicName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreateNetworkInterface", err)
		return armnetwork.Interface{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreateNetworkInterface", err)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	return resp.Interface, nil
}

func (c *LiveAzureClient) GetNetworkInterface(ctx context.Context, resourceGroupName, nicName string) (armnetwork.Interface, error) {
	logger.LogAzureAPIStart("GetNetworkInterface")
	resp, err := c.interfacesClient.Get(ctx, resourceGroupName, nicName, nil)
	logger.LogAzureAPIEnd("GetNetworkInterface", err)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	return resp.Interface, nil
}

func (c *LiveAzureClient) CreateNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
	logger.LogAzureAPIStart("CreateNetworkSecurityGroup")
	poller, err := c.securityGroupsClient.BeginCreateOrUpdate(ctx, resourceGroupName, sgName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreateNetworkSecurityGroup", err)
		return armnetwork.SecurityGroup{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreateNetworkSecurityGroup", err)
	if err != nil {
		return armnetwork.SecurityGroup{}, err
	}

	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) GetNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string) (armnetwork.SecurityGroup, error) {
	logger.LogAzureAPIStart("GetNetworkSecurityGroup")
	resp, err := c.securityGroupsClient.Get(ctx, resourceGroupName, sgName, nil)
	logger.LogAzureAPIEnd("GetNetworkSecurityGroup", err)
	if err != nil {
		return armnetwork.SecurityGroup{}, err
	}

	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) SearchResources(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (armresourcegraph.ClientResourcesResponse, error) {
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
		return armresourcegraph.ClientResourcesResponse{}, fmt.Errorf("failed to query resources: %v", err)
	}

	if res.Data == nil || len(res.Data.([]interface{})) == 0 {
		fmt.Println("No resources found")
		return armresourcegraph.ClientResourcesResponse{}, nil
	}

	return res, nil
}

func (c *LiveAzureClient) NewListPager(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (*runtime.Pager[armresources.ClientListResponse], error) {
	logger.LogAzureAPIStart("NewListPager")
	client := c.resourcesClient

	var filter string
	if resourceGroup != "" {
		filter = fmt.Sprintf("resourceGroup eq '%s'", resourceGroup)
	}
	for key, value := range tags {
		if value != nil {
			if filter != "" {
				filter += " and "
			}
			filter += fmt.Sprintf("tagName eq '%s' and tagValue eq '%s'", key, *value)
		}
	}

	pager := client.NewListPager(&armresources.ClientListOptions{
		Filter: &filter,
	})

	logger.LogAzureAPIEnd("NewListPager", nil)
	return pager, nil
}

var _ AzureClient = &LiveAzureClient{}
