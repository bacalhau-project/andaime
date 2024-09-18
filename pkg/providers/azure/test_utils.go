package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/stretchr/testify/mock"
)

// MockClient is a mock implementation of the Client interface
type MockClient struct {
	mock.Mock
}

// Implement all methods of Client interface here...

// NewMockClient creates a new MockClient with common expectations set
func NewMockClient() *MockClient {
	mockClient := new(MockClient)
	// Set up common expectations here
	return mockClient
}

// AssertResourceGroupCreated is a helper function to assert that a resource group was created
func AssertResourceGroupCreated(
	t *testing.T,
	mockClient *MockClient,
	expectedName, expectedLocation string,
) {
	mockClient.AssertCalled(
		t,
		"CreateResourceGroup",
		mock.Anything,
		expectedName,
		armresources.ResourceGroup{
			Location: to.Ptr(expectedLocation),
		},
	)
}

// NewTestContext creates a new context for testing
func NewTestContext() context.Context {
	return context.Background()
}

// More helper functions...
