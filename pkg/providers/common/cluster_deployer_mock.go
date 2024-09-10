package common

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/mock"
)

type MockClusterDeployer struct {
	mock.Mock
}

func (cd *MockClusterDeployer) ProvisionOrchestrator(
	ctx context.Context,
	machineName string,
) error {
	return cd.Called(ctx, machineName).Error(0)
}

func (cd *MockClusterDeployer) ProvisionWorker(
	ctx context.Context,
	machineName string,
) error {
	return cd.Called(ctx, machineName).Error(0)
}

func (cd *MockClusterDeployer) DeployWorker(ctx context.Context, machineName string) error {
	return cd.Called(ctx, machineName).Error(0)
}

func (cd *MockClusterDeployer) ProvisionPackagesOnMachine(
	ctx context.Context,
	machineName string,
) error {
	return cd.Called(ctx, machineName).Error(0)
}

func (cd *MockClusterDeployer) ProvisionSSH(ctx context.Context) error {
	return cd.Called(ctx).Error(0)
}

func (cd *MockClusterDeployer) WaitForAllMachinesToReachState(
	ctx context.Context,
	resourceType string,
	state models.ResourceState,
) error {
	return cd.Called(ctx, resourceType, state).Error(0)
}

func (cd *MockClusterDeployer) ProvisionBacalhau(
	ctx context.Context,
) error {
	return cd.Called(ctx).Error(0)
}
