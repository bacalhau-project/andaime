package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
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
	tags map[string]*string) ([]AzureResource, error) {
	args := m.Called(ctx, searchScope, subscriptionID, tags)
	return args.Get(0).([]AzureResource), args.Error(1)
}

func TestAzureProvider_SearchResources_ReturnsEmptySlice(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}

	searchScope := "subscriptionID"
	subscriptionID := "subscriptionID"
	tags := map[string]*string{"tag1": to.Ptr("value1")}

	mockClient.On("SearchResources", mock.Anything, mock.Anything, mock.Anything).Return(armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			Data: []AzureResource{},
		},
	}, nil)

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

	expectedQuery := `Resources 
	| project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
		properties, sku, identity, zones, plan, kind, managedBy, 
		provisioningState = tostring(properties.provisioningState)| where tags['key1'] == 'value1'`

	mockClient.On("Resources", ctx, expectedQuery, mock.Anything).Return(armresourcegraph.ClientResourcesResponse{}, nil)

	_, err := provider.SearchResources(ctx, subscriptionID, subscriptionID, tags)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}
