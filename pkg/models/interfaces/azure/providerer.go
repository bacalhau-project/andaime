package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
)

// AzureProviderer defines the interface for AzureProvider, embedding common.Providerer and adding Azure-specific methods.
type AzureProviderer interface {
	common_interface.Providerer

	GetAzureClient() AzureClienter
	SetAzureClient(client AzureClienter)

	// Resource Group Management
	PrepareResourceGroup(ctx context.Context) error
	GetOrCreateResourceGroup(
		ctx context.Context,
		rgName string,
		locationData string,
		tags map[string]string,
	) (*armresources.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context, resourceGroupName string) error
	ListAllResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error)

	// Resource Management
	DestroyResources(ctx context.Context, resourceGroupName string) error
	GetResources(
		ctx context.Context,
		resourceGroup string,
		tags map[string]*string,
	) ([]interface{}, error)
	ListAllResourcesInSubscription(
		ctx context.Context,
		tags map[string]*string,
	) ([]interface{}, error)
	PollResources(ctx context.Context) ([]interface{}, error)

	// Deployment Management
	CancelAllDeployments(ctx context.Context)
	AllMachinesComplete() bool

	// Cloud API
	GetSKUsByLocation(ctx context.Context, location string) ([]armcompute.ResourceSKU, error)
}
