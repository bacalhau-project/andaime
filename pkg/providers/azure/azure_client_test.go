package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAzureClient struct {
	mock.Mock
}

func (m *MockAzureClient) Resources(
	ctx context.Context,
	query string,
	opts *armresourcegraph.ClientResourcesOptions,
) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, query, opts)
	return args.Get(0).(armresourcegraph.ClientResourcesResponse), args.Error(1)
}

func (m *MockAzureClient) GetOrCreateResourceGroup(
	ctx context.Context,
	location, name string,
	tags map[string]*string,
) (*armresources.ResourceGroup, error) {
	args := m.Called(ctx, location, name, tags)
	return args.Get(0).(*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) DestroyResourceGroup(
	ctx context.Context,
	resourceGroupName string,
) error {
	args := m.Called(ctx, resourceGroupName)
	return args.Error(0)
}

func (m *MockAzureClient) DeleteDeployment(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
) error {
	args := m.Called(ctx, resourceGroupName, deploymentName)
	return args.Error(0)
}

func (m *MockAzureClient) GetResourceGroup(
	ctx context.Context,
	location,
	resourceGroupName string,
) (*armresources.ResourceGroup, error) {
	args := m.Called(ctx, location, resourceGroupName)
	return args.Get(0).(*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) GetResource(
	ctx context.Context,
	resourceGroupName string,
	resourceType string,
	resourceName string,
) (*armresources.GenericResource, error) {
	args := m.Called(ctx, resourceGroupName, resourceType, resourceName)
	return args.Get(0).(*armresources.GenericResource), args.Error(1)
}

func (m *MockAzureClient) GetDeploymentsClient() *armresources.DeploymentsClient {
	args := m.Called()
	return args.Get(0).(*armresources.DeploymentsClient)
}

func (m *MockAzureClient) ListAllResourceGroups(
	ctx context.Context,
) (map[string]string, error) {
	args := m.Called(ctx)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(map[string]string), args.Error(1)
}

func (m *MockAzureClient) ListAllResourcesInSubscription(
	ctx context.Context,
	subscriptionID string,
	tags map[string]*string,
) ([]interface{}, error) {
	args := m.Called(ctx, subscriptionID, tags)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]interface{}), args.Error(1)
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

func (m *MockAzureClient) GetVirtualMachine(
	ctx context.Context,
	resourceGroupName string,
	vmName string,
) (*armcompute.VirtualMachine, error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return args.Get(0).(*armcompute.VirtualMachine), args.Error(1)
}

func (m *MockAzureClient) GetNetworkInterface(
	ctx context.Context,
	resourceGroupName string,
	nicName string,
) (*armnetwork.Interface, error) {
	args := m.Called(ctx, resourceGroupName, nicName)
	return args.Get(0).(*armnetwork.Interface), args.Error(1)
}

func (m *MockAzureClient) GetPublicIPAddress(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddress *armnetwork.PublicIPAddress,
) (string, error) {
	args := m.Called(ctx, resourceGroupName, publicIPAddress)
	return args.String(0), args.Error(1)
}

func (m *MockAzureClient) GetResources(
	ctx context.Context,
	subscriptionID string,
	resourceGroupName string,
	tags map[string]*string,
) ([]interface{}, error) {
	args := m.Called(ctx, subscriptionID, resourceGroupName, tags)

	// Get the first argument as []armresources.GenericResource
	genericResources, ok := args.Get(0).([]armresources.GenericResource)
	if !ok {
		// If the type assertion fails, return an empty slice and the error
		return []interface{}{}, args.Error(1)
	}

	// Create a new slice of interface{}
	resources := make([]interface{}, len(genericResources))

	// Convert each GenericResource to interface{}
	for i, resource := range genericResources {
		resources[i] = resource
	}

	return resources, args.Error(1)
}

func (m *MockAzureClient) NewSubscriptionListPager(
	ctx context.Context,
	options *armsubscription.SubscriptionsClientListOptions,
) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	args := m.Called(ctx, options)
	return args.Get(0).(*runtime.Pager[armsubscription.SubscriptionsClientListResponse])
}

func (m *MockAzureClient) GetVMExtensions(
	ctx context.Context,
	resourceGroupName, vmName string,
) ([]*armcompute.VirtualMachineExtension, error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return args.Get(0).([]*armcompute.VirtualMachineExtension), args.Error(1)
}

func TestAzureProvider_ListAllResourceGroups(t *testing.T) {
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}
	ctx := context.Background()

	expectedResourceGroups := map[string]string{
		"test-rg": "eastus",
	}
	mockClient.On("ListAllResourceGroups", ctx).Return(expectedResourceGroups, nil)

	client := provider.GetAzureClient()
	resourceGroups, err := client.ListAllResourceGroups(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedResourceGroups, resourceGroups)
}

func TestAzureProvider_ListAllResourcesInSubscription_ReturnsEmptySlice(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}

	subscriptionID := "subscriptionID"
	tags := map[string]*string{"tag1": to.Ptr("value1")}

	// Mock the client to return nil resources and no error
	mockClient.On("ListAllResourcesInSubscription", mock.Anything, subscriptionID, tags).Return(
		nil, nil,
	)

	resources, err := provider.ListAllResourcesInSubscription(ctx, subscriptionID, tags)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{}, resources)
	mockClient.AssertExpectations(t)
}

func TestAzureProvider_ListAllResourcesInSubscription_NonStringTagValues(t *testing.T) {
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}
	ctx := context.Background()
	subscriptionID := "test-subscription-id"
	tags := map[string]*string{
		"key1": to.Ptr("value1"),
		"key2": nil,
	}

	// Test case 1: Return nil resources
	mockClient.On("ListAllResourcesInSubscription", ctx, subscriptionID, tags).
		Return(nil, nil).
		Once()

	resources, err := provider.ListAllResourcesInSubscription(ctx, subscriptionID, tags)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{}, resources)

	// Test case 2: Return error
	expectedError := fmt.Errorf("some error")
	mockClient.On("ListAllResourcesInSubscription", ctx, subscriptionID, tags).
		Return(nil, expectedError).
		Once()
	provider = &AzureProvider{Client: mockClient}

	resources, err = provider.ListAllResourcesInSubscription(ctx, subscriptionID, tags)
	assert.Error(t, err)
	assert.Equal(t, "failed to query resources: some error", err.Error())
	assert.Nil(t, resources)

	mockClient.AssertExpectations(t)
}

func TestAzureProvider_GetVirtualMachine(t *testing.T) {
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}
	ctx := context.Background()
	resourceGroupName := "test-rg"
	vmName := "test-vm"
	vm := &armcompute.VirtualMachine{
		Name: to.Ptr(vmName),
	}
	mockClient.On("GetVirtualMachine", ctx, resourceGroupName, vmName).Return(
		vm,
		nil,
	)

	client := provider.GetAzureClient()
	vm, err := client.GetVirtualMachine(ctx, resourceGroupName, vmName)
	assert.NoError(t, err)
	assert.NotNil(t, vm)

	mockClient.AssertExpectations(t)
}

// Test to make sure MockAzureClient implements AzureClient interface
var _ AzureClient = new(MockAzureClient)
