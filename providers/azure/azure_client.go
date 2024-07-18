//nolint:lll
package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

type AzureClient interface {
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
	SearchResources(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (armresourcegraph.ClientResourcesResponse, error)
}

// LiveAzureClient wraps all Azure SDK calls
type LiveAzureClient struct {
	virtualNetworksClient   *armnetwork.VirtualNetworksClient
	publicIPAddressesClient *armnetwork.PublicIPAddressesClient
	interfacesClient        *armnetwork.InterfacesClient
	virtualMachinesClient   *armcompute.VirtualMachinesClient
	securityGroupsClient    *armnetwork.SecurityGroupsClient
	resourceGraphClient     *armresourcegraph.Client
}

// NewAzureClient creates a new AzureClient
func NewAzureClient(subscriptionID string) (*LiveAzureClient, error) {
	// Get credential from CLI
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		// TODO: handle error
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

	return &LiveAzureClient{
		virtualNetworksClient:   virtualNetworksClient,
		publicIPAddressesClient: publicIPAddressesClient,
		interfacesClient:        interfacesClient,
		virtualMachinesClient:   virtualMachinesClient,
		securityGroupsClient:    securityGroupsClient,
		resourceGraphClient:     resourceGraphClient,
	}, nil
}

// CreateVirtualNetwork creates a new virtual network
func (c *LiveAzureClient) CreateVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
	poller, err := c.virtualNetworksClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, parameters, nil)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	return resp.VirtualNetwork, nil
}

// GetVirtualNetwork retrieves a virtual network
func (c *LiveAzureClient) GetVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string) (armnetwork.VirtualNetwork, error) {
	resp, err := c.virtualNetworksClient.Get(ctx, resourceGroupName, vnetName, nil)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	return resp.VirtualNetwork, nil
}

// CreatePublicIP creates a new public IP address
func (c *LiveAzureClient) CreatePublicIP(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
	poller, err := c.publicIPAddressesClient.BeginCreateOrUpdate(ctx, resourceGroupName, ipName, parameters, nil)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}

	return resp.PublicIPAddress, nil
}

func (c *LiveAzureClient) GetPublicIP(ctx context.Context, resourceGroupName, ipName string) (armnetwork.PublicIPAddress, error) {
	resp, err := c.publicIPAddressesClient.Get(ctx, resourceGroupName, ipName, nil)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}

	return resp.PublicIPAddress, nil
}

func (c *LiveAzureClient) CreateVirtualMachine(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
	poller, err := c.virtualMachinesClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, parameters, nil)
	if err != nil {
		return armcompute.VirtualMachine{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armcompute.VirtualMachine{}, err
	}

	return resp.VirtualMachine, nil
}

func (c *LiveAzureClient) GetVirtualMachine(ctx context.Context, resourceGroupName, vmName string) (armcompute.VirtualMachine, error) {
	resp, err := c.virtualMachinesClient.Get(ctx, resourceGroupName, vmName, nil)
	if err != nil {
		return armcompute.VirtualMachine{}, err
	}

	return resp.VirtualMachine, nil
}

func (c *LiveAzureClient) CreateNetworkInterface(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
	poller, err := c.interfacesClient.BeginCreateOrUpdate(ctx, resourceGroupName, nicName, parameters, nil)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	return resp.Interface, nil
}

func (c *LiveAzureClient) GetNetworkInterface(ctx context.Context, resourceGroupName, nicName string) (armnetwork.Interface, error) {
	resp, err := c.interfacesClient.Get(ctx, resourceGroupName, nicName, nil)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	return resp.Interface, nil
}

func (c *LiveAzureClient) CreateNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
	poller, err := c.securityGroupsClient.BeginCreateOrUpdate(ctx, resourceGroupName, sgName, parameters, nil)
	if err != nil {
		return armnetwork.SecurityGroup{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armnetwork.SecurityGroup{}, err
	}

	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) GetNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string) (armnetwork.SecurityGroup, error) {
	resp, err := c.securityGroupsClient.Get(ctx, resourceGroupName, sgName, nil)
	if err != nil {
		return armnetwork.SecurityGroup{}, err
	}

	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) SearchResources(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (armresourcegraph.ClientResourcesResponse, error) {
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
	if err != nil {
		return armresourcegraph.ClientResourcesResponse{}, fmt.Errorf("failed to query resources: %v", err)
	}

	if res.Data == nil {
		return armresourcegraph.ClientResourcesResponse{}, nil
	}

	return res, nil
}
