package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/stretchr/testify/mock"
)

// AssertResourceGroupCreated is a helper function to assert that a resource group was created
func AssertResourceGroupCreated(
	t *testing.T,
	mockClient *azure_mocks.MockAzureClienter,
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
