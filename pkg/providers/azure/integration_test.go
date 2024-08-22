package azure

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAzureClient struct {
	mock.Mock
}

func (m *MockAzureClient) DeployTemplate(ctx context.Context, resourceGroupName string, deploymentName string, template map[string]interface{}, params map[string]interface{}, tags map[string]*string) (interface{}, error) {
	args := m.Called(ctx, resourceGroupName, deploymentName, template, params, tags)
	return args.Get(0), args.Error(1)
}

func (m *MockAzureClient) GetResources(ctx context.Context, subscriptionID string, resourceGroupName string, tags map[string]*string) ([]interface{}, error) {
	args := m.Called(ctx, subscriptionID, resourceGroupName, tags)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockAzureClient) GetOrCreateResourceGroup(ctx context.Context, name string, location string, tags map[string]*string) (interface{}, error) {
	args := m.Called(ctx, name, location, tags)
	return args.Get(0), args.Error(1)
}

func (m *MockAzureClient) GetVirtualMachine(ctx context.Context, resourceGroupName string, vmName string) (interface{}, error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return args.Get(0), args.Error(1)
}

func (m *MockAzureClient) GetNetworkInterface(ctx context.Context, resourceGroupName string, nicName string) (interface{}, error) {
	args := m.Called(ctx, resourceGroupName, nicName)
	return args.Get(0), args.Error(1)
}

func (m *MockAzureClient) GetPublicIPAddress(ctx context.Context, resourceGroupName string, publicIPAddress interface{}) (string, error) {
	args := m.Called(ctx, resourceGroupName, publicIPAddress)
	return args.String(0), args.Error(1)
}

func TestAzureProviderIntegration(t *testing.T) {
	// Setup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mockAzureClient := new(MockAzureClient)
	mockSSHConfig := new(sshutils.MockSSHConfig)

	provider := &AzureProvider{
		azureClient: mockAzureClient,
	}

	m := display.GetGlobalModelFunc()
	m.Deployment = models.NewDeployment()
	m.Deployment.Machines = map[string]*models.Machine{
		"orchestrator": {Name: "orchestrator", Orchestrator: true},
		"worker1":      {Name: "worker1"},
		"worker2":      {Name: "worker2"},
	}

	// Mock resource creation
	mockAzureClient.On("GetOrCreateResourceGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAzureClient.On("DeployTemplate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAzureClient.On("GetResources", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]interface{}{}, nil)
	mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).Return(struct{}{}, nil)
	mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).Return(struct{}{}, nil)
	mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).Return("1.2.3.4", nil)

	// Mock SSH operations
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, privateKeyMaterial []byte) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)
	mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockSSHConfig.On("RestartService", mock.Anything, mock.Anything).Return(nil)

	// Run the test
	err := provider.DeployResources(ctx)
	assert.NoError(t, err)

	// Verify that all machines are provisioned
	for _, machine := range m.Deployment.Machines {
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("SSH"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Docker"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
	}

	// Test failure scenarios
	t.Run("Resource provisioning failure", func(t *testing.T) {
		mockAzureClient.On("DeployTemplate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("resource provisioning failed"))
		
		err := provider.DeployResources(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource provisioning failed")
	})

	t.Run("SSH provisioning failure", func(t *testing.T) {
		mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SSH provisioning failed"))
		
		err := provider.DeployResources(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SSH provisioning failed")
	})

	t.Run("Docker provisioning failure", func(t *testing.T) {
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "docker --version").Return("", fmt.Errorf("Docker not installed"))
		
		err := provider.DeployResources(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Docker not installed")
	})

	t.Run("Orchestrator provisioning failure", func(t *testing.T) {
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "bacalhau version").Return("", fmt.Errorf("Bacalhau not installed"))
		
		err := provider.DeployResources(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Bacalhau not installed")

		// Verify that worker provisioning is cancelled
		for _, machine := range m.Deployment.Machines {
			if !machine.Orchestrator {
				assert.NotEqual(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
			}
		}
	})

	// Verify error logging
	logEntries := logger.GetLastLines(10)
	assert.Contains(t, logEntries, "resource provisioning failed")
	assert.Contains(t, logEntries, "SSH provisioning failed")
	assert.Contains(t, logEntries, "Docker not installed")
	assert.Contains(t, logEntries, "Bacalhau not installed")
}
