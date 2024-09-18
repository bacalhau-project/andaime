package gcp

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/iam/v1"
)

type MockGCPProvider struct {
	common_interface.Providerer
	mock.Mock
}

func (m *MockGCPProvider) PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	args := m.Called(ctx)
	return args.Get(0).(*models.Deployment), args.Error(1)
}

func (m *MockGCPProvider) StartResourcePolling(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) PrepareResourceGroup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) CreateResources(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) CreateVM(
	ctx context.Context,
	machineName string,
) (models.Machiner, error) {
	args := m.Called(ctx, machineName)
	return args.Get(0).(models.Machiner), args.Error(1)
}

func (m *MockGCPProvider) CreateFirewallRules(ctx context.Context, machineName string) error {
	args := m.Called(ctx, machineName)
	return args.Error(0)
}

func (m *MockGCPProvider) EnsureFirewallRules(ctx context.Context, machineName string) error {
	args := m.Called(ctx, machineName)
	return args.Error(0)
}

func (m *MockGCPProvider) CreateServiceAccount(
	ctx context.Context,
	machineName string,
) (*iam.ServiceAccount, error) {
	args := m.Called(ctx, machineName)
	return args.Get(0).(*iam.ServiceAccount), args.Error(1)
}

func (m *MockGCPProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	args := m.Called()
	return args.Get(0).(common_interface.ClusterDeployerer)
}

func (m *MockGCPProvider) FinalizeDeployment(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) GetMachines(ctx context.Context) ([]models.Machine, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Machine), args.Error(1)
}

func (m *MockGCPProvider) GetServiceState(
	ctx context.Context,
	serviceName string,
) (models.ServiceState, error) {
	args := m.Called(ctx, serviceName)
	return args.Get(0).(models.ServiceState), args.Error(1)
}

func (m *MockGCPProvider) CheckPermissions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) CheckAuthentication(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) DestroyProject(ctx context.Context, projectID string) error {
	args := m.Called(ctx, projectID)
	return args.Error(0)
}

func (m *MockGCPProvider) EnableAPI(
	ctx context.Context,
	projectID, apiName string,
) error {
	args := m.Called(ctx, apiName)
	return args.Error(0)
}

func (m *MockGCPProvider) EnableRequiredAPIs(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) EnsureProject(ctx context.Context, projectID string) (string, error) {
	args := m.Called(ctx, projectID)
	return args.String(0), args.Error(1)
}

func (m *MockGCPProvider) GetGCPClient() gcp_interface.GCPClienter {
	args := m.Called()
	return args.Get(0).(gcp_interface.GCPClienter)
}

func (m *MockGCPProvider) SetGCPClient(client gcp_interface.GCPClienter) {
	m.Called(client)
}

func (m *MockGCPProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	m.Called(deployer)
}

func (m *MockGCPProvider) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]*assetpb.Asset), args.Error(1)
}

func (m *MockGCPProvider) ListProjects(
	ctx context.Context,
) ([]*resourcemanagerpb.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*resourcemanagerpb.Project), args.Error(1)
}

func (m *MockGCPProvider) SetBillingAccount(ctx context.Context, billingAccount string) error {
	args := m.Called(ctx, billingAccount)
	return args.Error(0)
}

func (m *MockGCPProvider) CreateServiceAccountKey(
	ctx context.Context,
	serviceAccountEmail string,
	keyType string,
) (*iam.ServiceAccountKey, error) {
	args := m.Called(ctx, serviceAccountEmail, keyType)
	return args.Get(0).(*iam.ServiceAccountKey), args.Error(1)
}

func (m *MockGCPProvider) CreateStorageBucket(
	ctx context.Context,
	bucketName string,
) error {
	args := m.Called(ctx, bucketName)
	return args.Error(0)
}

func (m *MockGCPProvider) ListBillingAccounts(
	ctx context.Context,
) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

var _ common_interface.Providerer = &MockGCPProvider{}
