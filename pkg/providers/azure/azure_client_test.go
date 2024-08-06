package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
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

func TestAzureProvider_SearchResources_ReturnsEmptySlice(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockAzureClient)
	provider := &AzureProvider{Client: mockClient}

	searchScope := "subscriptionID"
	subscriptionID := "subscriptionID"
	tags := map[string]string{"tag1": "value1"}

	mockClient.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(armresourcegraph.ClientResourcesResponse{
		Data: []interface{}{},
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
	tags := map[string]string{
		"key1": "value1",
		"key2": "",
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
func TestAzureClient_SearchResources_WithEmptyTagValues(t *testing.T) {
	subscriptionID := "test-subscription-id"
	expectedQuery := `Resources | project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, properties, sku, identity, zones, plan, kind, managedBy, provisioningState = tostring(properties.provisioningState)`

	mockResourceGraphClient := new(MockResourceGraphClient)

	ctx := context.Background()
	tags := map[string]*string{
		"key1": to.Ptr("value1"),
		"key2": nil, // Empty tag value
	}

	azureClient := &LiveAzureClient{
		resourceGraphClient: mockResourceGraphClient,
	}

	mockResourceGraphClient.On(
		"Resources",
		mock.Anything,
		armresourcegraph.QueryRequest{
			Query:         to.Ptr(expectedQuery + " | where tags['key1'] == 'value1'"),
			Subscriptions: []*string{to.Ptr(subscriptionID)},
		},
		mock.Anything,
	).Return(
		armresourcegraph.QueryResponse{
			Data: []interface{}{},
		},
		nil,
	)

	_, err := azureClient.SearchResources(ctx, subscriptionID, subscriptionID, tags)
	assert.NoError(t, err)
	mockResourceGraphClient.AssertExpectations(t)
}
func TestLiveAzureClient_SearchResources_NilTagValues(t *testing.T) {
	mockClient := &MockResourceGraphClient{}
	azureClient := &LiveAzureClient{
		resourceGraphClient: mockClient,
	}

	ctx := context.Background()
	subscriptionID := "subscription-id"
	searchScope := subscriptionID
	tags := map[string]*string{
		"key1": nil,
		"key2": to.Ptr("value2"),
	}

	expectedQuery := `Resources 
	| project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
		properties, sku, identity, zones, plan, kind, managedBy, 
		provisioningState = tostring(properties.provisioningState) | where tags['key2'] == 'value2'`

	mockResponse := armresourcegraph.QueryResponse{
		Data: []interface{}{
			map[string]interface{}{
				"id":       "/subscriptions/subscription-id/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm",
				"name":     "my-vm",
				"type":     "Microsoft.Compute/virtualMachines",
				"location": "eastus",
				"tags": map[string]*string{
					"key1": nil,
					"key2": to.Ptr("value2"),
				},
			},
		},
	}

	mockClient.On("Resources", ctx, armresourcegraph.QueryRequest{
		Query:         to.Ptr(expectedQuery),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}, nil).Return(mockResponse, nil)

	resources, err := azureClient.SearchResources(ctx, searchScope, subscriptionID, tags)
	assert.NoError(t, err)
	assert.Len(t, resources, 1)

	resource := resources[0]
	assert.Equal(t, "/subscriptions/subscription-id/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm", to.Ptr(resource.ID))
	assert.Equal(t, "my-vm", to.Ptr(resource.Name))
	assert.Equal(t, "Microsoft.Compute/virtualMachines", to.Ptr(resource.Type))
	assert.Equal(t, "eastus", to.Ptr(resource.Location))
	assert.Equal(t, tags, resource.Tags)

	mockClient.AssertExpectations(t)
}
func TestSearchResources_FilterByTagsWithSubscriptionID(t *testing.T) {
	ctx := context.Background()
	subscriptionID := "subscription-id"
	tags := map[string]*string{
		"tag1": to.Ptr("value1"),
		"tag2": to.Ptr("value2"),
	}

	// Set up mock resource graph client
	mockResourceGraphClient := new(MockResourceGraphClient)
	queryRequest := armresourcegraph.QueryRequest{
		Query: to.Ptr(`Resources 
			| project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
				properties, sku, identity, zones, plan, kind, managedBy, 
				provisioningState = tostring(properties.provisioningState)
			| where tags['tag1'] == 'value1'
			| where tags['tag2'] == 'value2'`),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}
	mockResponse := armresourcegraph.QueryResponse{
		Data: []interface{}{
			map[string]interface{}{
				"id":       "resource-id-1",
				"name":     "resource-name-1",
				"type":     "Resource.Type",
				"location": "westus",
				"tags": map[string]*string{
					"tag1": to.Ptr("value1"),
					"tag2": to.Ptr("value2"),
				},
			},
			map[string]interface{}{
				"id":       "resource-id-2",
				"name":     "resource-name-2",
				"type":     "Resource.Type",
				"location": "eastus",
				"tags": map[string]*string{
					"tag1": to.Ptr("value1"),
					"tag2": to.Ptr("value2"),
				},
			},
		},
	}

	mockResourceGraphClient.On("Resources", ctx, queryRequest, nil).Return(mockResponse, nil)

	// Create the LiveAzureClient with the mock resource graph client
	liveAzureClient := &LiveAzureClient{
		resourceGraphClient: mockResourceGraphClient,
	}

	// Call the SearchResources method
	resources, err := liveAzureClient.SearchResources(ctx, subscriptionID, subscriptionID, tags)
	assert.NoError(t, err)
	assert.Len(t, resources, 2)

	// Add additional assertions as needed
	mockResourceGraphClient.AssertExpectations(t)
}
func TestSearchResources_FilterByResourceGroup(t *testing.T) {
	mockClient := new(MockResourceGraphClient)
	client := &LiveAzureClient{resourceGraphClient: mockClient}
	ctx := context.Background()
	searchScope := "test-resource-group"
	subscriptionID := "test-subscription-id"
	tags := map[string]*string{}

	expectedQuery := `Resources 
	| project id, name, type, location, resourceGroup, subscriptionId, tenantId, tags, 
		properties, sku, identity, zones, plan, kind, managedBy, 
		provisioningState = tostring(properties.provisioningState) | where resourceGroup == 'test-resource-group'`
	expectedRequest := armresourcegraph.QueryRequest{
		Query:         to.Ptr(expectedQuery),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}
	expectedResponse := armresourcegraph.QueryResponse{
		Data: []armresources.GenericResource{
			{
				ID:       to.Ptr("test-resource-id"),
				Name:     to.Ptr("test-resource-name"),
				Type:     to.Ptr("test-resource-type"),
				Location: to.Ptr("test-location"),
			},
		},
	}

	mockClient.On("Resources", ctx, expectedRequest, nil).Return(expectedResponse, nil)

	resources, err := client.SearchResources(ctx, searchScope, subscriptionID, tags)
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "test-resource-id", *resources[0].ID)
	assert.Equal(t, "test-resource-name", *resources[0].Name)
	assert.Equal(t, "test-resource-type", *resources[0].Type)
	assert.Equal(t, "test-location", *resources[0].Location)

	mockClient.AssertExpectations(t)
}
