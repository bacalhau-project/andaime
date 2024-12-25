package gcp_interface

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/iam/v1"

	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
)

//go:generate mockery --name GCPClienter --output ../../../../mocks/gcp --outpkg gcp
type GCPClienter interface {
	common_interface.Clienter

	// GCP-specific methods
	ListProjects(
		ctx context.Context,
		req *resourcemanagerpb.ListProjectsRequest,
	) ([]*resourcemanagerpb.Project, error)

	EnsureProject(
		ctx context.Context,
		organizationID string,
		projectID string,
		billingAccountID string,
	) (string, error)
	DestroyProject(ctx context.Context, projectID string) error

	ListAllAssetsInProject(
		ctx context.Context,
		projectID string,
	) ([]*assetpb.Asset, error)
	ListAddresses(
		ctx context.Context,
		projectID string,
		region string,
	) ([]*computepb.Address, error)
	StartResourcePolling(ctx context.Context) error

	CheckAuthentication(ctx context.Context, projectID string) error
	CheckPermissions(ctx context.Context, projectID string) error
	EnableAPI(ctx context.Context, projectID, apiName string) error
	CreateVPCNetwork(ctx context.Context, projectID string, networkName string) error
	CreateFirewallRules(
		ctx context.Context,
		projectID string,
		networkName string,
	) error
	CreateIP(
		ctx context.Context,
		projectID string,
		location string,
		address *computepb.Address,
	) (*computepb.Address, error)
	DeleteIP(
		ctx context.Context,
		projectID string,
		location string,
		addressName string,
	) error
	// CreateStorageBucket(ctx context.Context, bucketName string) error
	CreateVM(
		ctx context.Context,
		projectID string,
		machine models.Machiner,
		ip *computepb.Address,
		networkName string,
	) (*computepb.Instance, error)
	SetBillingAccount(
		ctx context.Context,
		projectID string,
		billingAccountID string,
	) error
	ListBillingAccounts(ctx context.Context) ([]string, error)
	CreateServiceAccount(
		ctx context.Context,
		projectID string,
	) (*iam.ServiceAccount, error)
	IsAPIEnabled(ctx context.Context, projectID, apiName string) (bool, error)
	GetVMExternalIP(
		ctx context.Context,
		vmName string,
		locationData map[string]string,
	) (string, error)
	// WaitForGlobalOperation(
	// 	ctx context.Context,
	// 	project, operation string,
	// ) error
	GetVMZone(
		ctx context.Context,
		projectID, vmName string,
	) (string, error)
	CheckFirewallRuleExists(
		ctx context.Context,
		projectID, ruleName string,
	) error
	ValidateMachineType(
		ctx context.Context,
		projectID string,
		machineType, location string,
	) (bool, error)
	EnsureVPCNetwork(ctx context.Context, projectID, networkName string) error
	EnsureFirewallRules(
		ctx context.Context,
		projectID string,
		networkName string,
		allowedPorts []int,
	) error
	// EnsureStorageBucket(ctx context.Context, location, bucketName string) error
	ProjectExists(ctx context.Context, projectID string) (bool, error)

	GetParentString() string
	SetParentString(organizationID string)
}
