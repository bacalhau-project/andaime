// pkg/providers/common/interfaces.go
package common

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/models"
)

// Providerer defines the interface that all cloud providers must implement.
type Providerer interface {
	// Internal Tools
	PrepareDeployment(ctx context.Context) (*models.Deployment, error)
	StartResourcePolling(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error

	// Resource Management
	CreateResources(ctx context.Context) error
	DestroyResources(ctx context.Context, deploymentID string) error
	PollAndUpdateResources(ctx context.Context) ([]interface{}, error)
	GetVMExternalIP(
		ctx context.Context,
		vmName string,
		locationData map[string]string,
	) (string, error)

	// Cluster Deployer
	GetClusterDeployer() ClusterDeployerer
	SetClusterDeployer(deployer ClusterDeployerer)

	// Resource Validation
	ValidateMachineType(ctx context.Context, location, machineType string) (bool, error)

	// Azure specific methods
	PrepareResourceGroup(ctx context.Context) error
	GetResourceGroup(
		ctx context.Context,
		location, resourceGroupName string,
	) (*armresources.ResourceGroup, error)
	ListAllResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error)
	GetOrCreateResourceGroup(ctx context.Context) (*armresources.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context) error
	ListAllResourcesInSubscription(
		ctx context.Context,
		tags map[string]*string,
	) ([]interface{}, error)
	GetResources(
		ctx context.Context,
		resourceGroup string,
		tags map[string]*string,
	) ([]interface{}, error)
	GetSKUsByLocation(
		ctx context.Context,
		location string,
	) ([]armcompute.ResourceSKU, error)

	// GCP specific methods
	EnsureProject(
		ctx context.Context,
		projectID string,
	) (string, error)
	DestroyProject(
		ctx context.Context,
		projectID string,
	) error
	ListProjects(
		ctx context.Context,
	) ([]*resourcemanagerpb.Project, error)
	ListAllAssetsInProject(
		ctx context.Context,
		projectID string,
	) ([]*assetpb.Asset, error)
	SetBillingAccount(
		ctx context.Context,
		billingAccountID string,
	) error
	CheckAuthentication(ctx context.Context) error
	EnableRequiredAPIs(ctx context.Context) error
	CreateComputeInstance(ctx context.Context, machineName string) (models.Machiner, error)
	EnableAPI(ctx context.Context, projectID string, api string) error
	EnsureVPCNetwork(ctx context.Context, vpcNetworkName string) error
	EnsureFirewallRules(
		ctx context.Context,
		firewallRuleName string,
	) error
}

// ProviderFactory defines an interface to create providers.
type ProviderFactory interface {
	CreateProvider(ctx context.Context) (Providerer, error)
}
