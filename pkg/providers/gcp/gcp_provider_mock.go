package gcp

import (
	"context"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

type MockGCPProvider struct {
	mock.Mock
}

func (m *MockGCPProvider) StartResourcePolling(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockGCPProvider) PrepareResourceGroup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) CreateResources(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) GetClusterDeployer() common.ClusterDeployerer {
	args := m.Called()
	return args.Get(0).(common.ClusterDeployerer)
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

func (m *MockGCPProvider) CheckAuthentication(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGCPProvider) DestroyProject(ctx context.Context, projectID string) error {
	args := m.Called(ctx, projectID)
	return args.Error(0)
}

func (m *MockGCPProvider) EnableAPI(ctx context.Context, projectID string) error {
	args := m.Called(ctx, projectID)
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

func (m *MockGCPProvider) GetConfig() *viper.Viper {
	args := m.Called()
	return args.Get(0).(*viper.Viper)
}

func (m *MockGCPProvider) SetConfig(config *viper.Viper) {
	m.Called(config)
}

func (m *MockGCPProvider) GetSSHClient() sshutils.SSHClienter {
	args := m.Called()
	return args.Get(0).(sshutils.SSHClienter)
}

func (m *MockGCPProvider) SetSSHClient(client sshutils.SSHClienter) {
	m.Called(client)
}

func (m *MockGCPProvider) GetGCPClient() GCPClienter {
	args := m.Called()
	return args.Get(0).(GCPClienter)
}

func (m *MockGCPProvider) SetGCPClient(client GCPClienter) {
	m.Called(client)
}

func (m *MockGCPProvider) SetClusterDeployer(deployer common.ClusterDeployerer) {
	m.Called(deployer)
}

func (m *MockGCPProvider) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]*assetpb.Asset), args.Error(1)
}

func (m *MockGCPProvider) ListProjects(ctx context.Context) ([]*resourcemanagerpb.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*resourcemanagerpb.Project), args.Error(1)
}

func (m *MockGCPProvider) SetBillingAccount(ctx context.Context, billingAccount string) error {
	args := m.Called(ctx, billingAccount)
	return args.Error(0)
}

var _ GCPProviderer = &MockGCPProvider{}
