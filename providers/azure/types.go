package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

type ClientInterfaces struct {
	VirtualNetworksClient   VirtualNetworksClientAPI
	PublicIPAddressesClient PublicIPAddressesClientAPI
	NetworkInterfacesClient NetworkInterfacesClientAPI
	VirtualMachinesClient   VirtualMachinesClientAPI
	SecurityGroupsClient    SecurityGroupsClientAPI
	ResourceGraphClient     ResourceGraphClientAPI
}

type SSHWaiter interface {
	WaitForSSH(publicIP, username string, privateKey []byte) error
}

type PublicIPAddressesClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		publicIPAddressName string,
		parameters armnetwork.PublicIPAddress,
		options *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions,
	) (*runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error)

	Get(
		ctx context.Context,
		resourceGroupName string,
		publicIPAddressName string,
		options *armnetwork.PublicIPAddressesClientGetOptions,
	) (armnetwork.PublicIPAddressesClientGetResponse, error)
}

type NetworkInterfacesClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		networkInterfaceName string,
		parameters armnetwork.Interface,
		options *armnetwork.InterfacesClientBeginCreateOrUpdateOptions,
	) (*runtime.Poller[armnetwork.InterfacesClientCreateOrUpdateResponse], error)
}

// VirtualNetworksClientAPI defines the methods we need from VirtualNetworksClient
type VirtualNetworksClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		virtualNetworkName string,
		parameters armnetwork.VirtualNetwork,
		options *armnetwork.VirtualNetworksClientBeginCreateOrUpdateOptions,
	) (*runtime.Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse], error)
	Get(
		ctx context.Context,
		resourceGroupName string,
		virtualNetworkName string,
		options *armnetwork.VirtualNetworksClientGetOptions,
	) (armnetwork.VirtualNetworksClientGetResponse, error)
}

// VirtualMachinesClientAPI defines the methods we need from VirtualMachinesClient
type VirtualMachinesClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		vmName string,
		parameters armcompute.VirtualMachine,
		options *armcompute.VirtualMachinesClientBeginCreateOrUpdateOptions,
	) (*runtime.Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse], error)
}

type ResourceGraphClientAPI interface {
	Resources(
		ctx context.Context,
		request armresourcegraph.QueryRequest,
		options *armresourcegraph.ClientResourcesOptions,
	) (armresourcegraph.ClientResourcesResponse, error)
}

// AzureResource represents a single Azure resource
type AzureResource struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	ID       string            `json:"id"`
	Location string            `json:"location"`
	Tags     map[string]string `json:"tags"`
}

// QueryResponse represents the response from an Azure Resource Graph query
type QueryResponse struct {
	Count           *int64                                 `json:"count"`
	Data            []AzureResource                        `json:"data"`
	Facets          []armresourcegraph.FacetClassification `json:"facets"`
	ResultTruncated *armresourcegraph.ResultTruncated      `json:"resultTruncated"`
	TotalRecords    *int64                                 `json:"totalRecords"`
}
