package gcp

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
)

// GCPProviderer defines the interface for GCP-specific provider operations
type GCPProviderer interface {
	common_interface.Providerer

	// Authentication and Client Management
	GetGCPClient() GCPClienter
	SetGCPClient(client GCPClienter)
	CheckAuthentication(ctx context.Context) error

	// Project Management
	EnsureProject(ctx context.Context, projectID string) (string, error)
	DestroyProject(ctx context.Context, projectID string) error
	ListProjects(ctx context.Context) ([]*resourcemanagerpb.Project, error)
	ListAllAssetsInProject(ctx context.Context, projectID string) ([]*assetpb.Asset, error)

	// Deployment Management
	PrepareDeployment(ctx context.Context) (*models.Deployment, error)
	FinalizeDeployment(ctx context.Context) error

	// API Management
	EnableRequiredAPIs(ctx context.Context) error
	EnableAPI(ctx context.Context, apiName string) error

	// Network Management
	CreateVPCNetwork(ctx context.Context, networkName string) error
	EnsureVPCNetwork(ctx context.Context, vpcNetworkName string) error
	CreateFirewallRules(ctx context.Context, networkName string) error
	EnsureFirewallRules(ctx context.Context, networkName string) error

	// Storage Management
	CreateStorageBucket(ctx context.Context, bucketName string) error

	// Billing Management
	ListBillingAccounts(ctx context.Context) ([]string, error)
	SetBillingAccount(ctx context.Context, billingAccountID string) error

	// VM Management
	CreateVM(ctx context.Context, vmName string) (string, string, error)
}
