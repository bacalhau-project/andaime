package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// Ensure GCPProvider implements ClusterDeployer
var _ common.ClusterDeployer = (*GCPProvider)(nil)

// CreateResources implements the ClusterDeployer interface for GCP
func (p *GCPProvider) CreateResources(ctx context.Context) error {
	// TODO: Implement GCP-specific resource creation
	return fmt.Errorf("CreateResources not implemented for GCP")
}

// ProvisionSSH implements the ClusterDeployer interface for GCP
func (p *GCPProvider) ProvisionSSH(ctx context.Context) error {
	// TODO: Implement GCP-specific SSH provisioning
	return fmt.Errorf("ProvisionSSH not implemented for GCP")
}

// SetupDocker implements the ClusterDeployer interface for GCP
func (p *GCPProvider) SetupDocker(ctx context.Context) error {
	// TODO: Implement GCP-specific Docker setup
	return fmt.Errorf("SetupDocker not implemented for GCP")
}

// DeployOrchestrator implements the ClusterDeployer interface for GCP
func (p *GCPProvider) DeployOrchestrator(ctx context.Context) error {
	// TODO: Implement GCP-specific orchestrator deployment
	return fmt.Errorf("DeployOrchestrator not implemented for GCP")
}

// DeployNodes implements the ClusterDeployer interface for GCP
func (p *GCPProvider) DeployNodes(ctx context.Context) error {
	// TODO: Implement GCP-specific node deployment
	return fmt.Errorf("DeployNodes not implemented for GCP")
}

func (p *GCPProvider) DeployCluster(ctx context.Context) error {
	return fmt.Errorf("DeployCluster not implemented for GCP")
}

// Update GCPProviderer interface to include ClusterDeployer methods
type GCPProviderer interface {
	common.ClusterDeployer
	GetGCPClient() GCPClienter
	SetGCPClient(client GCPClienter)

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
	SetBillingAccount(ctx context.Context) error
	DeployResources(ctx context.Context) error
	ProvisionPackagesOnMachines(ctx context.Context) error
	ProvisionBacalhau(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	StartResourcePolling(ctx context.Context)
	CheckAuthentication(ctx context.Context) error
	EnableAPI(ctx context.Context, apiName string) error
	EnableRequiredAPIs(ctx context.Context) error
	CreateVPCNetwork(
		ctx context.Context,
		networkName string,
	) error
	CreateFirewallRules(
		ctx context.Context,
		networkName string,
	) error
	CreateStorageBucket(
		ctx context.Context,
		bucketName string,
	) error
	CreateVM(
		ctx context.Context,
		projectID string,
		vmConfig map[string]string,
	) (string, error)
	ListBillingAccounts(ctx context.Context) ([]string, error)
	GetVMExternalIP(ctx context.Context, projectID, zone, vmName string) (string, error)
}
