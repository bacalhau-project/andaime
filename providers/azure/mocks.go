package azure

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

func getClientInterfaces() ClientInterfaces {
	return ClientInterfaces{
		VirtualNetworksClient:   &MockVirtualNetworksClient{},
		PublicIPAddressesClient: &MockPublicIPAddressesClient{},
		NetworkInterfacesClient: &MockNetworkInterfacesClient{},
		VirtualMachinesClient:   &MockVirtualMachinesClient{},
		SecurityGroupsClient:    &MockSecurityGroupsClient{},
		ResourceGraphClient:     &MockResourceGraphClient{},
	}
}

type MockPoller[T any] struct {
	result T
}

func (m *MockPoller[T]) Done() bool {
	return true
}

func (m *MockPoller[T]) Poll(ctx context.Context) (*http.Response, error) {
	return &http.Response{}, nil
}

func (m *MockPoller[T]) FinalResponse(ctx context.Context) (*http.Response, error) {
	return &http.Response{}, nil
}

func (m *MockPoller[T]) Result(ctx context.Context) (T, error) {
	return m.result, nil
}

func (m *MockPoller[T]) PollUntilDone(ctx context.Context, options *runtime.PollUntilDoneOptions) (T, error) {
	return m.result, nil
}

// Add these new methods to match runtime.Poller interface
func (m *MockPoller[T]) ResumeToken() (string, error) {
	return "", nil
}

// NewMockPoller creates a new MockPoller that satisfies the *runtime.Poller interface
func NewMockPoller[T any](result T) *runtime.Poller[T] {
	return (*runtime.Poller[T])(&MockPoller[T]{result: result})
}

func NewMockClientInterfaces() *ClientInterfaces {
	return &ClientInterfaces{
		VirtualNetworksClient:   &MockVirtualNetworksClient{},
		PublicIPAddressesClient: &MockPublicIPAddressesClient{},
		NetworkInterfacesClient: &MockNetworkInterfacesClient{},
		VirtualMachinesClient:   &MockVirtualMachinesClient{},
		SecurityGroupsClient:    &MockSecurityGroupsClient{},
	}
}

// Mock client interfaces
type MockPublicIPAddressesClient struct {
	IPAddress string
	tags      map[string]*string
}

func (m *MockPublicIPAddressesClient) BeginCreateOrUpdate(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddressName string,
	parameters armnetwork.PublicIPAddress,
	options *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions,
) (*runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error) {
	result := armnetwork.PublicIPAddressesClientCreateOrUpdateResponse{
		PublicIPAddress: parameters,
	}
	return NewMockPoller(result), nil
}
func (m *MockPublicIPAddressesClient) Get(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddressName string,
	options *armnetwork.PublicIPAddressesClientGetOptions,
) (armnetwork.PublicIPAddressesClientGetResponse, error) {
	return armnetwork.PublicIPAddressesClientGetResponse{
		PublicIPAddress: armnetwork.PublicIPAddress{
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				IPAddress: to.Ptr(m.IPAddress),
			},
		},
	}, nil
}

type MockNetworkInterfacesClient struct {
	tags map[string]*string
}

func (m *MockNetworkInterfacesClient) BeginCreateOrUpdate(
	ctx context.Context,
	resourceGroupName string,
	networkInterfaceName string,
	parameters armnetwork.Interface,
	options *armnetwork.InterfacesClientBeginCreateOrUpdateOptions,
) (*runtime.Poller[armnetwork.InterfacesClientCreateOrUpdateResponse], error) {
	result := armnetwork.InterfacesClientCreateOrUpdateResponse{
		Interface: parameters,
	}

	return NewMockPoller(result), nil
}

type MockVirtualMachinesClient struct {
	tags map[string]*string
}

func (m *MockVirtualMachinesClient) BeginCreateOrUpdate(
	ctx context.Context,
	resourceGroupName string,
	vmName string,
	parameters armcompute.VirtualMachine,
	options *armcompute.VirtualMachinesClientBeginCreateOrUpdateOptions,
) (*runtime.Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse], error) {
	result := armcompute.VirtualMachinesClientCreateOrUpdateResponse{
		VirtualMachine: parameters,
	}
	return NewMockPoller(result), nil
}

type MockSecurityGroupsClient struct {
	tags map[string]*string
}

func (m *MockSecurityGroupsClient) BeginCreateOrUpdate(
	ctx context.Context,
	resourceGroupName string,
	networkSecurityGroupName string,
	parameters armnetwork.SecurityGroup,
	options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions,
) (*runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse], error) {
	result := armnetwork.SecurityGroupsClientCreateOrUpdateResponse{
		SecurityGroup: parameters,
	}
	return NewMockPoller(result), nil
}

type MockVirtualNetworksClient struct {
	tags map[string]*string
}

func (m *MockVirtualNetworksClient) BeginCreateOrUpdate(
	ctx context.Context,
	resourceGroupName string,
	virtualNetworkName string,
	parameters armnetwork.VirtualNetwork,
	options *armnetwork.VirtualNetworksClientBeginCreateOrUpdateOptions,
) (*runtime.Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse], error) {
	result := armnetwork.VirtualNetworksClientCreateOrUpdateResponse{
		VirtualNetwork: parameters,
	}
	return NewMockPoller(result), nil
}

func (m *MockVirtualNetworksClient) Get(
	ctx context.Context,
	resourceGroupName string,
	virtualNetworkName string,
	options *armnetwork.VirtualNetworksClientGetOptions,
) (armnetwork.VirtualNetworksClientGetResponse, error) {
	// Mock implementation
	return armnetwork.VirtualNetworksClientGetResponse{
		VirtualNetwork: armnetwork.VirtualNetwork{
			Properties: &armnetwork.VirtualNetworkPropertiesFormat{
				Subnets: []*armnetwork.Subnet{
					{
						Name: to.Ptr("test-subnet"),
						Properties: &armnetwork.SubnetPropertiesFormat{
							AddressPrefix: to.Ptr("10.0.0.0/24"),
						},
					},
				},
			},
		},
	}, nil
}

type MockResourceGraphClient struct {
	Response armresourcegraph.ClientResourcesResponse
}

func (m *MockResourceGraphClient) Resources(
	ctx context.Context,
	request armresourcegraph.QueryRequest,
	options *armresourcegraph.ClientResourcesOptions,
) (armresourcegraph.ClientResourcesResponse, error) {
	return m.Response, nil
}

func MockQueryResponse() armresourcegraph.ClientResourcesResponse {
	return armresourcegraph.ClientResourcesResponse{QueryResponse: armresourcegraph.QueryResponse{
		Count: to.Ptr[int64](3), //nolint:gomnd
		Data: []AzureResource{
			{
				Name:     "myNetworkInterface",
				Type:     "microsoft.network/networkinterfaces",
				ID:       "/subscriptions/cfbbd179-59d2-4052-aa06-9270a38aa9d6/resourceGroups/RG1/providers/Microsoft.Network/networkInterfaces/myNetworkInterface", //nolint:lll
				Location: "centralus",
				Tags:     map[string]string{"tag1": "Value1"},
			},
			{
				Name:     "myVnet",
				Type:     "microsoft.network/virtualnetworks",
				ID:       "/subscriptions/cfbbd179-59d2-4052-aa06-9270a38aa9d6/resourceGroups/RG2/providers/Microsoft.Network/virtualNetworks/myVnet", //nolint:lll
				Location: "westus",
				Tags:     map[string]string{},
			},
			{
				Name:     "myPublicIp",
				Type:     "microsoft.network/publicipaddresses",
				ID:       "/subscriptions/cfbbd179-59d2-4052-aa06-9270a38aa9d6/resourceGroups/RG2/providers/Microsoft.Network/publicIPAddresses/myPublicIp", //nolint:lll
				Location: "westus",
				Tags:     map[string]string{},
			},
		},
		Facets:          []armresourcegraph.FacetClassification{},
		ResultTruncated: to.Ptr(armresourcegraph.ResultTruncatedFalse),
		TotalRecords:    to.Ptr[int64](3), //nolint:gomnd
	},
	}
}

var _ runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse] = &MockPoller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse]{}
var _ runtime.Poller[armnetwork.InterfacesClientCreateOrUpdateResponse] = &MockPoller[armnetwork.InterfacesClientCreateOrUpdateResponse]{}
var _ runtime.Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse] = &MockPoller[armcompute.VirtualMachinesClientCreateOrUpdateResponse]{}
var _ runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse] = &MockPoller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse]{}
var _ runtime.Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse] = &MockPoller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse]{}
