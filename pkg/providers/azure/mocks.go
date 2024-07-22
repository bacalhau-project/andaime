//nolint:lll
package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/stretchr/testify/mock"
)

type MockAzureClient struct {
	mock.Mock
	Logger                         *logger.Logger
	CreateVirtualNetworkFunc       func(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error)
	GetVirtualNetworkFunc          func(ctx context.Context, resourceGroupName, vnetName string) (armnetwork.VirtualNetwork, error)
	CreatePublicIPFunc             func(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error)
	GetPublicIPFunc                func(ctx context.Context, resourceGroupName, ipName string) (armnetwork.PublicIPAddress, error)
	CreateVirtualMachineFunc       func(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error)
	GetVirtualMachineFunc          func(ctx context.Context, resourceGroupName, vmName string) (armcompute.VirtualMachine, error)
	CreateNetworkInterfaceFunc     func(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error)
	GetNetworkInterfaceFunc        func(ctx context.Context, resourceGroupName, nicName string) (armnetwork.Interface, error)
	CreateNetworkSecurityGroupFunc func(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error)
	GetNetworkSecurityGroupFunc    func(ctx context.Context, resourceGroupName, sgName string) (armnetwork.SecurityGroup, error)
	SearchResourcesFunc            func(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (armresourcegraph.ClientResourcesResponse, error)
}

func NewMockAzureClient() AzureClient {
	return &MockAzureClient{}
}

func (m *MockAzureClient) CreateVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
	return m.CreateVirtualNetworkFunc(ctx, resourceGroupName, vnetName, parameters)
}

func (m *MockAzureClient) GetVirtualNetwork(ctx context.Context, resourceGroupName, vnetName string) (armnetwork.VirtualNetwork, error) {
	return m.GetVirtualNetworkFunc(ctx, resourceGroupName, vnetName)
}

func (m *MockAzureClient) CreatePublicIP(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
	return m.CreatePublicIPFunc(ctx, resourceGroupName, ipName, parameters)
}

func (m *MockAzureClient) GetPublicIP(ctx context.Context, resourceGroupName, ipName string) (armnetwork.PublicIPAddress, error) {
	return m.GetPublicIPFunc(ctx, resourceGroupName, ipName)
}

func (m *MockAzureClient) CreateVirtualMachine(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
	return m.CreateVirtualMachineFunc(ctx, resourceGroupName, vmName, parameters)
}

func (m *MockAzureClient) GetVirtualMachine(ctx context.Context, resourceGroupName, vmName string) (armcompute.VirtualMachine, error) {
	return m.GetVirtualMachineFunc(ctx, resourceGroupName, vmName)
}

func (m *MockAzureClient) CreateNetworkInterface(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
	return m.CreateNetworkInterfaceFunc(ctx, resourceGroupName, nicName, parameters)
}

func (m *MockAzureClient) GetNetworkInterface(ctx context.Context, resourceGroupName, nicName string) (armnetwork.Interface, error) {
	return m.GetNetworkInterfaceFunc(ctx, resourceGroupName, nicName)
}

func (m *MockAzureClient) CreateNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
	return m.CreateNetworkSecurityGroupFunc(ctx, resourceGroupName, sgName, parameters)
}

func (m *MockAzureClient) GetNetworkSecurityGroup(ctx context.Context, resourceGroupName, sgName string) (armnetwork.SecurityGroup, error) {
	return m.GetNetworkSecurityGroupFunc(ctx, resourceGroupName, sgName)
}

func (m *MockAzureClient) SearchResources(ctx context.Context, resourceGroup string, tags map[string]*string, subscriptionID string) (armresourcegraph.ClientResourcesResponse, error) {
	return m.SearchResourcesFunc(ctx, resourceGroup, tags, subscriptionID)
}

type MockAzureProvider struct {
	mock.Mock
}

func (m *MockAzureProvider) CreateDeployment(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

var GetMockAzureProviderFunc = GetMockAzureProvider

func GetMockAzureProvider() (*MockAzureProvider, error) {
	return &MockAzureProvider{}, nil
}
