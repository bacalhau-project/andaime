package gcp_interface

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/iam/v1"

	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
)

type GCPClienter interface {
	common_interface.Clienter

	// GCP-specific methods
	ListProjects(
		ctx context.Context,
		req *resourcemanagerpb.ListProjectsRequest,
	) ([]*resourcemanagerpb.Project, error)

	EnsureProject(
		ctx context.Context,
		projectID string,
	) (string, error)
	DestroyProject(ctx context.Context, projectID string) error

	ListAllAssetsInProject(
		ctx context.Context,
		projectID string,
	) ([]*assetpb.Asset, error)
	StartResourcePolling(ctx context.Context) error

	CheckAuthentication(ctx context.Context) error
	CheckPermissions(ctx context.Context) error
	EnableAPI(ctx context.Context, projectID, apiName string) error
	CreateVPCNetwork(ctx context.Context, networkName string) error
	CreateFirewallRules(ctx context.Context, networkName string) error
	CreateStorageBucket(ctx context.Context, bucketName string) error
	CreateVM(
		ctx context.Context,
		vmName string,
	) (*computepb.Instance, error)
	WaitForOperation(
		ctx context.Context,
		project, zone, operation string,
	) error
	SetBillingAccount(ctx context.Context, billingAccountID string) error
	ListBillingAccounts(ctx context.Context) ([]string, error)
	CreateServiceAccount(
		ctx context.Context,
		projectID string,
	) (*iam.ServiceAccount, error)
	CreateServiceAccountKey(
		ctx context.Context,
		projectID, serviceAccountEmail string,
	) (*iam.ServiceAccountKey, error)
	WaitForRegionalOperation(
		ctx context.Context,
		project, region, operation string,
	) error
	IsAPIEnabled(ctx context.Context, projectID, apiName string) (bool, error)
	GetVMExternalIP(
		ctx context.Context,
		vmName string,
		locationData map[string]string,
	) (string, error)
	WaitForGlobalOperation(
		ctx context.Context,
		project, operation string,
	) error
	GetVMZone(
		ctx context.Context,
		projectID, vmName string,
	) (string, error)
	CheckFirewallRuleExists(
		ctx context.Context,
		projectID, ruleName string,
	) error
	ValidateMachineType(ctx context.Context, machineType, location string) (bool, error)
	EnsureVPCNetwork(ctx context.Context, networkName string) error
	EnsureFirewallRules(ctx context.Context, networkName string) error
	// EnsureStorageBucket(ctx context.Context, location, bucketName string) error
}
