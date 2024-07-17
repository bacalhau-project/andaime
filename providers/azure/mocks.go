package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/google/uuid"
)

func getClientInterfaces() ClientInterfaces {
	return ClientInterfaces{
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
) (Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error) {
	parameters.ID = to.Ptr(uuid.New().String())
	parameters.Properties.IPAddress = to.Ptr(m.IPAddress)

	resp := armnetwork.PublicIPAddressesClientCreateOrUpdateResponse{
		PublicIPAddress: parameters,
	}
	return &MockPoller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse]{result: resp}, nil
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
) (Poller[armnetwork.InterfacesClientCreateOrUpdateResponse], error) {
	m.tags = parameters.Tags
	mockNIC := armnetwork.Interface{
		ID:       to.Ptr(uuid.New().String()),
		Name:     to.Ptr("testNIC"),
		Location: to.Ptr("eastus"),
		Tags: map[string]*string{
			"andaime":                   to.Ptr("true"),
			"andaime-id":                to.Ptr("uuid"),
			"andaime-project":           to.Ptr("uuid-testProject"),
			"unique-id":                 to.Ptr("uuid"),
			"project-id":                to.Ptr("testProject"),
			"deployed-by":               to.Ptr("andaime"),
			"andaime-resource-tracking": to.Ptr("true"),
		},
	}
	resp := armnetwork.InterfacesClientCreateOrUpdateResponse{
		Interface: mockNIC,
	}
	return &MockPoller[armnetwork.InterfacesClientCreateOrUpdateResponse]{result: resp}, nil
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
) (Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse], error) {
	m.tags = parameters.Tags
	return &MockPoller[armcompute.VirtualMachinesClientCreateOrUpdateResponse]{
		result: armcompute.VirtualMachinesClientCreateOrUpdateResponse{VirtualMachine: parameters},
	}, nil
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
) (Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse], error) {
	resp := armnetwork.VirtualNetworksClientCreateOrUpdateResponse{
		VirtualNetwork: parameters,
	}
	return &MockPoller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse]{result: resp}, nil
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
) (Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse], error) {
	mockParams := &armnetwork.SecurityGroup{
		ID:       to.Ptr(uuid.New().String()),
		Name:     to.Ptr("testNSG"),
		Location: to.Ptr("eastus"),
		Tags: map[string]*string{
			"andaime":                   to.Ptr("true"),
			"andaime-id":                to.Ptr("uuid"),
			"andaime-project":           to.Ptr("uuid-testProject"),
			"unique-id":                 to.Ptr("uuid"),
			"project-id":                to.Ptr("testProject"),
			"deployed-by":               to.Ptr("andaime"),
			"andaime-resource-tracking": to.Ptr("true"),
		},
	}
	return &MockPoller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse]{
		result: armnetwork.SecurityGroupsClientCreateOrUpdateResponse{SecurityGroup: *mockParams},
	}, nil
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

type MockSSHClient struct {
	CloseCalled bool
}

func (m *MockSSHClient) Close() error {
	m.CloseCalled = true
	return nil
}
