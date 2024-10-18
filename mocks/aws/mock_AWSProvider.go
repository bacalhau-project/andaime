package mocks

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/stretchr/testify/mock"
)

type MockAWSProvider struct {
	mock.Mock
}

func (m *MockAWSProvider) CreateDeployment(ctx context.Context, instanceType aws.InstanceType) error {
	args := m.Called(ctx, instanceType)
	return args.Error(0)
}

func (m *MockAWSProvider) TerminateDeployment(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAWSProvider) ListInstances(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}
