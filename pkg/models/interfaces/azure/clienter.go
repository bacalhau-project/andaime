package azure_interface

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"

	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
)

//go:generate mockery --name AzureClienter --output ../../../../mocks/azure --outpkg azure
// AzureClienter defines the methods that any Azure client must implement.
// This abstraction allows for easier testing and flexibility in client implementations.
type AzureClienter interface {
	common_interface.Clienter

	// ListResourceGroups lists all resource groups in the subscription.
	ListAllResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error)

	// GetResourceGroup retrieves a specific resource group by name and location.
	GetOrCreateResourceGroup(
		ctx context.Context,
		resourceGroupName string,
		location string,
		tags map[string]string,
	) (*armresources.ResourceGroup, error)

	// DestroyResourceGroup deletes the specified resource group.
	DestroyResourceGroup(ctx context.Context, resourceGroupName string) error

	// GetVMExternalIP retrieves the external IP address of a specified VM.
	GetVMExternalIP(
		ctx context.Context,
		vmName string,
		locationData map[string]string,
	) (string, error)

	// DeployTemplate deploys an ARM template to a resource group.
	DeployTemplate(
		ctx context.Context,
		resourceGroupName string,
		deploymentName string,
		template map[string]interface{},
		params map[string]interface{},
		tags map[string]*string,
	) (Pollerer, error)

	// NewSubscriptionListPager returns a new pager for listing subscriptions.
	NewSubscriptionListPager(
		ctx context.Context,
		options *armsubscription.SubscriptionsClientListOptions,
	) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]

	// ListAllResourcesInSubscription lists all resources within a subscription based on provided tags.
	ListAllResourcesInSubscription(
		ctx context.Context,
		subscriptionID string,
		tags map[string]*string,
	) ([]interface{}, error)

	// GetResources retrieves resources based on subscription ID, resource group name, and tags.
	GetResources(
		ctx context.Context,
		subscriptionID string,
		resourceGroupName string,
		tags map[string]*string,
	) ([]interface{}, error)

	// GetVirtualMachine retrieves details of a specific virtual machine.
	GetVirtualMachine(
		ctx context.Context,
		resourceGroupName string,
		vmName string,
	) (*armcompute.VirtualMachine, error)

	// GetNetworkInterface retrieves details of a specific network interface.
	GetNetworkInterface(
		ctx context.Context,
		resourceGroupName string,
		networkInterfaceName string,
	) (*armnetwork.Interface, error)

	// GetPublicIPAddress retrieves the public IP address associated with a network interface.
	GetPublicIPAddress(
		ctx context.Context,
		resourceGroupName string,
		publicIPAddress *armnetwork.PublicIPAddress,
	) (string, error)

	// GetSKUsByLocation retrieves available SKUs in a specific location.
	GetSKUsByLocation(ctx context.Context, location string) ([]armcompute.ResourceSKU, error)

	// ValidateMachineType checks if the specified VM size is available in the given location.
	ValidateMachineType(ctx context.Context, location string, vmSize string) (bool, error)

	// ResourceGroupExists checks if a resource group exists.
	ResourceGroupExists(ctx context.Context, resourceGroupName string) (bool, error)
}
