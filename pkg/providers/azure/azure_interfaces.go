package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

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

	ListAllResourceGroups(
		ctx context.Context,
	) (map[string]string, error)

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

	// New methods for ARM template deployment
	DeployTemplate(
		ctx context.Context,
		resourceGroupName string,
		deploymentName string,
		template map[string]interface{},
		parameters map[string]interface{},
		tags map[string]*string,
	) (Pollerer, error)
	GetDeploymentsClient() *armresources.DeploymentsClient
	GetVirtualMachine(
		ctx context.Context,
		resourceGroupName string,
		vmName string,
	) (*armcompute.VirtualMachine, error)
	GetPublicIPAddress(
		ctx context.Context,
		resourceGroupName string,
		publicIP *armnetwork.PublicIPAddress,
	) (string, error)
	GetNetworkInterface(
		ctx context.Context,
		resourceGroupName string,
		networkInterfaceName string,
	) (*armnetwork.Interface, error)

	GetSKUsByLocation(
		ctx context.Context,
		location string,
	) ([]armcompute.ResourceSKU, error)

	ValidateMachineType(
		ctx context.Context,
		location string,
		vmSize string,
	) (bool, error)

	ResourceGroupExists(
		ctx context.Context,
		resourceGroupName string,
	) (bool, error)

	GetVMExternalIP(
		ctx context.Context,
		resourceGroupName, vmName string,
	) (string, error)
}
type AzureProviderer interface {
	common.Providerer
	GetAzureClient() AzureClienter
	SetAzureClient(client AzureClienter)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)
	GetClusterDeployer() common.ClusterDeployerer
	SetClusterDeployer(deployer common.ClusterDeployerer)

	PrepareResourceGroup(ctx context.Context) error

	DestroyResources(ctx context.Context, resourceGroupName string) error
	PollAndUpdateResources(ctx context.Context) ([]interface{}, error)
	GetVMExternalIP(ctx context.Context, resourceGroupName, vmName string) (string, error)
}
