// azure_interfaces.go
package azure

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// AzureClienter defines the interface for Azure client operations.
type AzureClienter interface {
	// Resource Group API
	GetOrCreateResourceGroup(
		ctx context.Context,
		location, name string,
		tags map[string]*string,
	) (*armresources.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context, resourceGroupName string) error

	// Subscriptions API
	NewSubscriptionListPager(
		ctx context.Context,
		options *armsubscription.SubscriptionsClientListOptions,
	) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]

	ListAllResourceGroups(ctx context.Context) (map[string]string, error)

	GetResourceGroup(
		ctx context.Context,
		location, name string,
	) (*armresources.ResourceGroup, error)

	ListAllResourcesInSubscription(
		ctx context.Context,
		subscriptionID string,
		tags map[string]*string,
	) ([]interface{}, error)

	GetResources(
		ctx context.Context,
		subscriptionID string,
		resourceGroupName string,
		tags map[string]*string,
	) ([]interface{}, error)

	// ARM Template Deployment API
	DeployTemplate(
		ctx context.Context,
		resourceGroupName, deploymentName string,
		template, parameters map[string]interface{},
		tags map[string]*string,
	) (Pollerer, error)
	GetDeploymentsClient() *armresources.DeploymentsClient

	// Virtual Machine API
	GetVirtualMachine(
		ctx context.Context,
		resourceGroupName, vmName string,
	) (*armcompute.VirtualMachine, error)
	GetPublicIPAddress(
		ctx context.Context,
		resourceGroupName string,
		publicIP *armnetwork.PublicIPAddress,
	) (string, error)
	GetNetworkInterface(
		ctx context.Context,
		resourceGroupName, networkInterfaceName string,
	) (*armnetwork.Interface, error)

	// SKU and Validation API
	GetSKUsByLocation(ctx context.Context, location string) ([]armcompute.ResourceSKU, error)
	ValidateMachineType(ctx context.Context, location, vmSize string) (bool, error)

	// Existence Check
	ResourceGroupExists(ctx context.Context, resourceGroupName string) (bool, error)

	// VM External IP
	GetVMExternalIP(ctx context.Context, resourceGroupName, vmName string) (string, error)

	// List Resource Groups
	ListResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error)
}

// AzureProviderer defines the interface for AzureProvider, embedding common.Providerer and adding Azure-specific methods.
type AzureProviderer interface {
	common.Providerer

	GetAzureClient() AzureClienter
	SetAzureClient(client AzureClienter)
	GetClusterDeployer() common.ClusterDeployerer
	SetClusterDeployer(deployer common.ClusterDeployerer)

	// Resource Group/Project Management
	PrepareResourceGroup(ctx context.Context) error
	DestroyResourceGroup(ctx context.Context) error

	DestroyResources(ctx context.Context, resourceGroupName string) error
	PollAndUpdateResources(ctx context.Context) ([]interface{}, error)
}

type Pollerer interface {
	PollUntilDone(
		ctx context.Context,
		options *runtime.PollUntilDoneOptions,
	) (armresources.DeploymentsClientCreateOrUpdateResponse, error)
	ResumeToken() (string, error)
	Result(ctx context.Context) (armresources.DeploymentsClientCreateOrUpdateResponse, error)
	Done() bool
	Poll(ctx context.Context) (*http.Response, error)
}
