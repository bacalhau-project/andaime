package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAzureClient struct {
	mock.Mock
}

func (m *MockAzureClient) Resources(ctx context.Context, query string, opts *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, query, opts)
	return args.Get(0).(armresourcegraph.ClientResourcesResponse), args.Error(1)
}

func (m *MockAzureClient) GetOrCreateResourceGroup(ctx context.Context, location, name string, tags map[string]*string) (*armresources.ResourceGroup, error) {
	args := m.Called(ctx, location, name, tags)
	return args.Get(0).(*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) DestroyResourceGroup(ctx context.Context, resourceGroupName string) error {
	args := m.Called(ctx, resourceGroupName)
	return args.Error(0)
}

func (m *MockAzureClient) DeployTemplate(ctx context.Context, resourceGroupName string, deploymentName string, template map[string]interface{}, parameters map[string]interface{}, tags map[string]*string) (*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse], error) {
	args := m.Called(ctx, resourceGroupName, deploymentName, template, parameters, tags)
	return args.Get(0).(*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse]), args.Error(1)
}

func (m *MockAzureClient) DeleteDeployment(ctx context.Context, resourceGroupName string, deploymentName string) error {
	args := m.Called(ctx, resourceGroupName, deploymentName)
	return args.Error(0)
}

func (m *MockAzureClient) GetResourceGroup(ctx context.Context, resourceGroupName string) (*armresources.ResourceGroup, error) {
	args := m.Called(ctx, resourceGroupName)
	return args.Get(0).(*armresources.ResourceGroup), args.Error(1)
}

func (m *MockAzureClient) GetResource(ctx context.Context, resourceGroupName string, resourceType string, resourceName string) (*armresources.GenericResource, error) {
	args := m.Called(ctx, resourceGroupName, resourceType, resourceName)
	return args.Get(0).(*armresources.GenericResource), args.Error(1)
}

func (m *MockAzureClient) GetDeploymentsClient() *armresources.DeploymentsClient {
	args := m.Called()
	return args.Get(0).(*armresources.DeploymentsClient)
}

func (m *MockAzureClient) ListResourcesInGroup(ctx context.Context, resourceGroupName string) ([]AzureResource, error) {
	args := m.Called(ctx, resourceGroupName)
	return args.Get(0).([]AzureResource), args.Error(1)
}

func (m *MockAzureClient) NewSubscriptionListPager(
	ctx context.Context,
	options *armsubscription.SubscriptionsClientListOptions,
) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	args := m.Called(ctx, options)
	return args.Get(0).(*runtime.Pager[armsubscription.SubscriptionsClientListResponse])
}

func (m *MockAzureClient) SearchResources(
	ctx context.Context,
	searchScope string,
	subscriptionID string,
	tags map[string]*string) ([]armresources.GenericResource, error) {
	args := m.Called(ctx, searchScope, subscriptionID, tags)
	return args.Get(0).([]armresources.GenericResource), args.Error(1)
}

func (m *MockAzureClient) GetVMExtensions(ctx context.Context, resourceGroupName, vmName string) ([]*armcompute.VirtualMachineExtension, error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return args.Get(0).([]*armcompute.VirtualMachineExtension), args.Error(1)
}

func TestAzureProvider_updateVMExtensionsStatus(t *testing.T) {
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}

	deployment := &models.Deployment{
		ResourceGroupName: "test-rg",
		VMExtensionsStatus: make(map[string]models.StatusCode),
	}

	resource := &armresources.GenericResource{
		Name: to.Ptr("test-vm/ext1"),
		Properties: map[string]interface{}{
			"provisioningState": "Succeeded",
		},
	}

	provider.updateVMExtensionsStatus(deployment, resource)

	assert.Equal(t, models.StatusSucceeded, deployment.VMExtensionsStatus["test-vm/ext1"])

	mockClient.AssertExpectations(t)
}
func TestAzureProvider_SearchResources_ReturnsEmptySlice(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}

	searchScope := "subscriptionID"
	subscriptionID := "subscriptionID"
	tags := map[string]*string{"tag1": to.Ptr("value1")}

	// Update this line to match the actual method signature
	mockClient.On("SearchResources", mock.Anything, searchScope, subscriptionID, tags).Return(
		[]armresources.GenericResource{}, nil,
	)

	resources, err := provider.SearchResources(ctx, searchScope, subscriptionID, tags)

	assert.NoError(t, err)
	assert.Empty(t, resources)
	mockClient.AssertExpectations(t)
}

func TestAzureProvider_SearchResources_NonStringTagValues(t *testing.T) {
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}
	ctx := context.Background()
	subscriptionID := "test-subscription-id"
	tags := map[string]*string{
		"key1": to.Ptr("value1"),
		"key2": nil,
	}

	// Mock the SearchResources method instead of Resources
	mockClient.On("SearchResources", ctx, subscriptionID, subscriptionID, tags).Return(
		[]armresources.GenericResource{}, nil,
	)

	_, err := provider.SearchResources(ctx, subscriptionID, subscriptionID, tags)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}
