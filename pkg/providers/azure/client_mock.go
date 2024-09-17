package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/mock"
)

type MockAzureClient struct {
	mock.Mock
}

func NewMockAzureClient() *MockAzureClient {
	return new(MockAzureClient)
}

func (m *MockAzureClient) DeployTemplate(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
	template map[string]interface{},
	parameters map[string]interface{},
	tags map[string]*string,
) (Pollerer, error) {
	args := m.Called(ctx, resourceGroupName, deploymentName, template, parameters, tags)
	return args.Get(0).(Pollerer), args.Error(1)
}

func (m *MockAzureClient) DestroyResourceGroup(
	ctx context.Context,
	resourceGroupName string,
) error {
	args := m.Called(ctx, resourceGroupName)
	return args.Error(0)
}

func (m *MockAzureClient) ListAllResourcesInSubscription(
	ctx context.Context,
	subscriptionID string,
	tags map[string]*string,
) ([]interface{}, error) {
	args := m.Called(ctx, subscriptionID, tags)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockAzureClient) ListAvailableSkus(
	ctx context.Context,
	location string,
) ([]string, error) {
	args := m.Called(ctx, location)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAzureClient) ValidateMachineType(
	ctx context.Context,
	location, machineType string,
) (bool, error) {
	args := m.Called(ctx, location, machineType)
	return args.Bool(0), args.Error(1)
}

func (m *MockAzureClient) GetResourceGroup(
	ctx context.Context,
	resourceGroupName string,
	location string,
) (*armresources.ResourceGroup, error) {
	args := m.Called(ctx, resourceGroupName, location)
	return args.Get(0).(*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) GetDeploymentsClient() *armresources.DeploymentsClient {
	return m.Called().Get(0).(*armresources.DeploymentsClient)
}

func (m *MockAzureClient) GetNetworkInterface(
	ctx context.Context,
	resourceGroupName, networkInterfaceName string,
) (*armnetwork.Interface, error) {
	args := m.Called(ctx, resourceGroupName, networkInterfaceName)
	return args.Get(0).(*armnetwork.Interface), args.Error(1)
}

func (m *MockAzureClient) GetOrCreateResourceGroup(
	ctx context.Context,
	resourceGroupName string,
	location string,
	tags map[string]string,
) (*armresources.ResourceGroup, error) {
	args := m.Called(ctx, resourceGroupName, location, tags)
	return args.Get(0).(*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) GetPublicIPAddress(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddress *armnetwork.PublicIPAddress,
) (string, error) {
	args := m.Called(ctx, resourceGroupName, publicIPAddress)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockAzureClient) GetResources(
	ctx context.Context,
	resourceGroupName string,
	resourceType string,
	tags map[string]*string,
) ([]interface{}, error) {
	args := m.Called(ctx, resourceGroupName, resourceType, tags)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockAzureClient) GetSKUsByLocation(
	ctx context.Context,
	location string,
) ([]armcompute.ResourceSKU, error) {
	args := m.Called(ctx, location)
	return args.Get(0).([]armcompute.ResourceSKU), args.Error(1)
}

func (m *MockAzureClient) GetVirtualMachine(
	ctx context.Context,
	resourceGroupName string,
	vmName string,
) (*armcompute.VirtualMachine, error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return args.Get(0).(*armcompute.VirtualMachine), args.Error(1)
}

func (m *MockAzureClient) ListAllResourceGroups(
	ctx context.Context,
) ([]*armresources.ResourceGroup, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) NewSubscriptionListPager(
	ctx context.Context,
	opts *armsubscription.SubscriptionsClientListOptions,
) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	return m.Called(ctx).Get(0).(*runtime.Pager[armsubscription.SubscriptionsClientListResponse])
}

func (m *MockAzureClient) ResourceGroupExists(
	ctx context.Context,
	resourceGroupName string,
) (bool, error) {
	args := m.Called(ctx, resourceGroupName)
	return args.Bool(0), args.Error(1)
}

func (m *MockAzureClient) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	args := m.Called(ctx, vmName, locationData)
	return args.Get(0).(string), args.Error(1)
}
