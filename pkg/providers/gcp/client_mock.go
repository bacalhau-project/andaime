package gcp

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iam/v1"
)

// MockGCPClient is a mock implementation of the GCPClienter interface
type MockGCPClient struct {
	mock.Mock
}

func (m *MockGCPClient) CheckAuthentication(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) CreateProject(ctx context.Context, projectID string) error {
	args := m.Called(ctx, projectID)
	return args.Error(0)
}

func (m *MockGCPClient) EnableRequiredAPIs(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) CreateNetwork(
	ctx context.Context,
	networkName string,
) (*compute.Network, error) {
	args := m.Called(ctx, networkName)
	return args.Get(0).(*compute.Network), args.Error(1)
}

func (m *MockGCPClient) CreateSubnetwork(
	ctx context.Context,
	subnetworkName, networkName, region, ipCidrRange string,
) (*compute.Subnetwork, error) {
	args := m.Called(ctx, subnetworkName, networkName, region, ipCidrRange)
	return args.Get(0).(*compute.Subnetwork), args.Error(1)
}

func (m *MockGCPClient) CreateFirewallRule(
	ctx context.Context,
	firewallRuleName, networkName string,
	allowed []*compute.FirewallAllowed,
	sourceRanges []string,
) (*compute.Firewall, error) {
	args := m.Called(ctx, firewallRuleName, networkName, allowed, sourceRanges)
	return args.Get(0).(*compute.Firewall), args.Error(1)
}

func (m *MockGCPClient) CreateInstance(
	ctx context.Context,
	instance *compute.Instance,
) (*compute.Operation, error) {
	args := m.Called(ctx, instance)
	return args.Get(0).(*compute.Operation), args.Error(1)
}

func (m *MockGCPClient) GetInstance(
	ctx context.Context,
	zone, instanceName string,
) (*compute.Instance, error) {
	args := m.Called(ctx, zone, instanceName)
	return args.Get(0).(*compute.Instance), args.Error(1)
}

func (m *MockGCPClient) DeleteInstance(
	ctx context.Context,
	zone, instanceName string,
) (*compute.Operation, error) {
	args := m.Called(ctx, zone, instanceName)
	return args.Get(0).(*compute.Operation), args.Error(1)
}

func (m *MockGCPClient) WaitForOperation(ctx context.Context, operation *compute.Operation) error {
	args := m.Called(ctx, operation)
	return args.Error(0)
}

func (m *MockGCPClient) GetExternalIP(
	ctx context.Context,
	zone, instanceName string,
) (string, error) {
	args := m.Called(ctx, zone, instanceName)
	return args.String(0), args.Error(1)
}

func (m *MockGCPClient) GetMachineType(
	ctx context.Context,
	zone, machineType string,
) (*compute.MachineType, error) {
	args := m.Called(ctx, zone, machineType)
	return args.Get(0).(*compute.MachineType), args.Error(1)
}

func (m *MockGCPClient) ListMachineTypes(
	ctx context.Context,
	zone string,
) ([]*compute.MachineType, error) {
	args := m.Called(ctx, zone)
	return args.Get(0).([]*compute.MachineType), args.Error(1)
}

func (m *MockGCPClient) ListZones(ctx context.Context) ([]*compute.Zone, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*compute.Zone), args.Error(1)
}

func (m *MockGCPClient) ListInstances(
	ctx context.Context,
	zone string,
) ([]*compute.Instance, error) {
	args := m.Called(ctx, zone)
	return args.Get(0).([]*compute.Instance), args.Error(1)
}

func (m *MockGCPClient) GetInstanceStatus(
	ctx context.Context,
	zone, instanceName string,
) (models.MachineResourceState, error) {
	args := m.Called(ctx, zone, instanceName)
	return args.Get(0).(models.MachineResourceState), args.Error(1)
}

func (m *MockGCPClient) DeleteNetwork(
	ctx context.Context,
	networkName string,
) (*compute.Operation, error) {
	args := m.Called(ctx, networkName)
	return args.Get(0).(*compute.Operation), args.Error(1)
}

func (m *MockGCPClient) DeleteSubnetwork(
	ctx context.Context,
	region, subnetworkName string,
) (*compute.Operation, error) {
	args := m.Called(ctx, region, subnetworkName)
	return args.Get(0).(*compute.Operation), args.Error(1)
}

func (m *MockGCPClient) DeleteFirewallRule(
	ctx context.Context,
	firewallRuleName string,
) (*compute.Operation, error) {
	args := m.Called(ctx, firewallRuleName)
	return args.Get(0).(*compute.Operation), args.Error(1)
}

func (m *MockGCPClient) ListNetworks(ctx context.Context) ([]*compute.Network, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*compute.Network), args.Error(1)
}

func (m *MockGCPClient) ListSubnetworks(
	ctx context.Context,
	region string,
) ([]*compute.Subnetwork, error) {
	args := m.Called(ctx, region)
	return args.Get(0).([]*compute.Subnetwork), args.Error(1)
}

func (m *MockGCPClient) ListFirewallRules(ctx context.Context) ([]*compute.Firewall, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*compute.Firewall), args.Error(1)
}

func (m *MockGCPClient) CreateComputeInstance(
	ctx context.Context,
	instanceName string,
) (*computepb.Instance, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(*computepb.Instance), args.Error(1)
}

func (m *MockGCPClient) CreateFirewallRules(
	ctx context.Context,
	firewallRule string,
) error {
	args := m.Called(ctx, firewallRule)
	return args.Error(0)
}

func (m *MockGCPClient) CreateServiceAccount(
	ctx context.Context,
	serviceAccountName string,
) (*iam.ServiceAccount, error) {
	args := m.Called(ctx, serviceAccountName)
	return args.Get(0).(*iam.ServiceAccount), args.Error(1)
}

func (m *MockGCPClient) CreateServiceAccountKey(
	ctx context.Context,
	serviceAccountName, keyType string,
) (*iam.ServiceAccountKey, error) {
	args := m.Called(ctx, serviceAccountName, keyType)
	return args.Get(0).(*iam.ServiceAccountKey), args.Error(1)
}

func (m *MockGCPClient) CreateStorageBucket(
	ctx context.Context,
	bucketName string,
) error {
	args := m.Called(ctx, bucketName)
	return args.Error(0)
}

func (m *MockGCPClient) CreateVPCNetwork(
	ctx context.Context,
	networkName string,
) error {
	args := m.Called(ctx, networkName)
	return args.Error(0)
}

func (m *MockGCPClient) CreateResources(
	ctx context.Context,
) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) DestroyProject(
	ctx context.Context,
	projectID string,
) error {
	args := m.Called(ctx, projectID)
	return args.Error(0)
}

func (m *MockGCPClient) EnableAPI(
	ctx context.Context,
	apiName, projectID string,
) error {
	args := m.Called(ctx, apiName, projectID)
	return args.Error(0)
}

func (m *MockGCPClient) EnsureFirewallRules(
	ctx context.Context,
	firewallRules string,
) error {
	args := m.Called(ctx, firewallRules)
	return args.Error(0)
}

func (m *MockGCPClient) EnsureProject(
	ctx context.Context,
	projectID string,
) (string, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockGCPClient) EnsureSubnetwork(
	ctx context.Context,
	subnetworkName, networkName, region, ipCidrRange string,
) (string, error) {
	args := m.Called(ctx, subnetworkName, networkName, region, ipCidrRange)
	return args.String(0), args.Error(1)
}

func (m *MockGCPClient) EnsureNetwork(
	ctx context.Context,
	networkName string,
) (string, error) {
	args := m.Called(ctx, networkName)
	return args.String(0), args.Error(1)
}

func (m *MockGCPClient) EnsureStorageBucket(
	ctx context.Context,
	bucketName, projectID string,
) error {
	args := m.Called(ctx, bucketName, projectID)
	return args.Error(0)
}

func (m *MockGCPClient) EnsureServiceAccount(
	ctx context.Context,
	serviceAccountName, projectID string,
) error {
	args := m.Called(ctx, serviceAccountName, projectID)
	return args.Error(0)
}

func (m *MockGCPClient) EnsureVPCNetwork(
	ctx context.Context,
	networkName string,
) error {
	args := m.Called(ctx, networkName)
	return args.Error(0)
}

func (m *MockGCPClient) FinalizeDeployment(
	ctx context.Context,
) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	args := m.Called(ctx, vmName, locationData)
	return args.String(0), args.Error(1)
}

func (m *MockGCPClient) IsAPIEnabled(
	ctx context.Context,
	apiName, projectID string,
) (bool, error) {
	args := m.Called(ctx, apiName, projectID)
	return args.Bool(0), args.Error(1)
}

func (m *MockGCPClient) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]*assetpb.Asset), args.Error(1)
}

func (m *MockGCPClient) ListBillingAccounts(
	ctx context.Context,
) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGCPClient) ListProjects(
	ctx context.Context,
	filter *resourcemanagerpb.ListProjectsRequest,
) ([]*resourcemanagerpb.Project, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*resourcemanagerpb.Project), args.Error(1)
}

func (m *MockGCPClient) ProvisionPackagesOnMachines(
	ctx context.Context,
) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) SetBillingAccount(
	ctx context.Context,
	billingAccountID string,
) error {
	args := m.Called(ctx, billingAccountID)
	return args.Error(0)
}

func (m *MockGCPClient) waitForOperation(
	ctx context.Context,
	project, zone, operation string,
) error {
	args := m.Called(ctx, project, zone, operation)
	return args.Error(0)
}

func (m *MockGCPClient) waitForRegionalOperation(
	ctx context.Context,
	project, region, operation string,
) error {
	args := m.Called(ctx, project, region, operation)
	return args.Error(0)
}

func (m *MockGCPClient) StartResourcePolling(
	ctx context.Context,
) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) ValidateMachineType(
	ctx context.Context,
	zone, machineType string,
) (bool, error) {
	args := m.Called(ctx, zone, machineType)
	return args.Bool(0), args.Error(1)
}

func (m *MockGCPClient) checkFirewallRuleExists(
	ctx context.Context,
	firewallRuleName string,
	networkName string,
) error {
	args := m.Called(ctx, firewallRuleName, networkName)
	return args.Error(0)
}

func (m *MockGCPClient) getVMZone(
	ctx context.Context,
	instanceName string,
	projectID string,
) (string, error) {
	args := m.Called(ctx, instanceName, projectID)
	return args.String(0), args.Error(1)
}

func (m *MockGCPClient) waitForGlobalOperation(
	ctx context.Context,
	project, operation string,
) error {
	args := m.Called(ctx, project, operation)
	return args.Error(0)
}

func (m *MockGCPClient) CheckPermissions(
	ctx context.Context,
) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPClient) CreateVM(
	ctx context.Context,
	vmName string,
) (*computepb.Instance, error) {
	args := m.Called(ctx, vmName)
	return args.Get(0).(*computepb.Instance), args.Error(1)
}
