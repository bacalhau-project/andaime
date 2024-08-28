package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/stretchr/testify/mock"
)

type MockAzureClient struct {
	mock.Mock
}

func (m *MockAzureClient) DeployTemplate(ctx context.Context, resourceGroupName string, deploymentName string, template map[string]interface{}, parameters map[string]interface{}, tags map[string]*string) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, resourceGroupName, deploymentName, template, parameters, tags)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockAzureClient) DestroyResourceGroup(ctx context.Context, resourceGroupName string) error {
	args := m.Called(ctx, resourceGroupName)
	return args.Error(0)
}

func (m *MockAzureClient) ListAllResourcesInSubscription(ctx context.Context, subscriptionID string, tags map[string]*string) ([]interface{}, error) {
	args := m.Called(ctx, subscriptionID, tags)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockAzureClient) ListAvailableSkus(ctx context.Context, location string) ([]string, error) {
	args := m.Called(ctx, location)
	return args.Get(0).([]string), args.Error(1)
}
