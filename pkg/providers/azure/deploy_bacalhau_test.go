package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockSSHConfig is a mock implementation of sshutils.SSHConfiger
type mockSSHConfig struct {
	mock.Mock
}

func (m *mockSSHConfig) Connect() (sshutils.SSHClienter, error)   { return nil, nil }
func (m *mockSSHConfig) Close() error                             { return nil }
func (m *mockSSHConfig) SetSSHClient(client sshutils.SSHClienter) {}
func (m *mockSSHConfig) PushFile(content []byte, remotePath string, executable bool) error {
	return m.Called(content, remotePath, executable).Error(0)
}
func (m *mockSSHConfig) ExecuteCommand(cmd string) (string, error) {
	args := m.Called(cmd)
	return args.String(0), args.Error(1)
}
func (m *mockSSHConfig) InstallSystemdService(serviceName string, servicePath string) error {
	return nil
}
func (m *mockSSHConfig) RestartService(serviceName string) error { return nil }
func (m *mockSSHConfig) StartService(serviceName string) error   { return nil }

// setupMockSSHConfig sets up the mock SSH config with expected calls
func setupMockSSHConfig(m *mockSSHConfig, expectedCalls int) {
	m.On("PushFile", mock.Anything, mock.Anything, true).Return(nil).Times(expectedCalls)
	m.On("ExecuteCommand", mock.Anything).Return("", nil).Times(expectedCalls)
	m.On("ExecuteCommand", "bacalhau node list --output json --api-host 0.0.0.0").
		Return(`[{"id": "node1"}]`, nil).
		Times(expectedCalls)
}

// TestCase represents a generic test case structure
type TestCase struct {
	name           string
	machines       []*models.Machine
	mockServices   []models.ServiceType
	mockSetup      func(*mockSSHConfig)
	expectedError  string
	expectedStates map[int]models.ServiceState
}

// runTest is a generic test runner for both orchestrator and worker tests
func runTest(t *testing.T, tc TestCase, testFunc func(*AzureProvider, context.Context) error) {
	t.Run(tc.name, func(t *testing.T) {
		m := display.GetGlobalModel()
		m.Deployment.Machines = tc.machines

		for i := range m.Deployment.Machines {
			for _, service := range tc.mockServices {
				m.Deployment.Machines[i].SetServiceState(service.Name, service.State)
			}
		}

		mockSSH := new(mockSSHConfig)
		if tc.mockSetup != nil {
			tc.mockSetup(mockSSH)
		}

		originalNewSSHConfig := sshutils.NewSSHConfigFunc
		sshutils.NewSSHConfigFunc = func(host string, port int, user string, privateKeyMaterial []byte) (sshutils.SSHConfiger, error) {
			if tc.expectedError == "Error creating SSH config" {
				return nil, assert.AnError
			}
			return mockSSH, nil
		}
		defer func() { sshutils.NewSSHConfigFunc = originalNewSSHConfig }()

		provider := &AzureProvider{}
		err := testFunc(provider, context.Background())

		assertTestResults(t, tc, err, m)
		mockSSH.AssertExpectations(t)
	})
}

// assertTestResults checks the test results against expected values
func assertTestResults(t *testing.T, tc TestCase, err error, m *display.DisplayModel) {
	if tc.expectedError != "" {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), tc.expectedError)
	} else {
		assert.NoError(t, err)
	}

	for i, expectedState := range tc.expectedStates {
		assert.Equal(t, expectedState, m.Deployment.Machines[i].GetServiceState("Bacalhau"))
	}
}

func TestDeployBacalhauOrchestrator(t *testing.T) {
	tests := []TestCase{
		{
			name:          "No orchestrator node and no orchestrator IP",
			machines:      []*models.Machine{},
			expectedError: "no orchestrator node found",
		},
		{
			name:          "No orchestrator node, but orchestrator IP specified",
			machines:      []*models.Machine{{OrchestratorIP: "NOT_EMPTY"}},
			expectedError: "orchestrator IP set",
		},
		{
			name: "One orchestrator node, no other machines",
			machines: []*models.Machine{
				{
					Orchestrator: true,
				},
			},
			mockServices: []models.ServiceType{
				{Name: "Bacalhau", State: models.ServiceStateUnknown},
			},
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Return(nil).Times(3)
				m.On("ExecuteCommand", mock.Anything).Return("", nil).Times(3)
				m.On("ExecuteCommand", "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node1"}]`, nil)
			},
		},
		{
			name: "One orchestrator node and many other machines",
			machines: []*models.Machine{
				{
					Orchestrator: true,
				},
				{},
				{},
			},
			mockServices: []models.ServiceType{
				{Name: "Bacalhau", State: models.ServiceStateUnknown},
			},
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Return(nil).Times(3)
				m.On("ExecuteCommand", mock.Anything).Return("", nil).Times(3)
				m.On("ExecuteCommand", "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node1"}]`, nil)
			},
		},
		{
			name:     "Multiple orchestrator nodes (should error)",
			machines: []*models.Machine{{Orchestrator: true}, {Orchestrator: true}},
			mockServices: []models.ServiceType{
				{Name: "Bacalhau", State: models.ServiceStateUnknown},
			},
			expectedError: "multiple orchestrator nodes found",
		},
	}

	for _, tc := range tests {
		runTest(t, tc, func(p *AzureProvider, ctx context.Context) error {
			return p.DeployBacalhauOrchestrator(ctx)
		})
	}
}

func TestDeployBacalhauOrchestrator_FailureScenarios(t *testing.T) {
	tests := []TestCase{
		{
			name: "Fail after Docker install",
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Return(nil)
				m.On("ExecuteCommand", mock.Anything).
					Return("", fmt.Errorf("Docker installation failed"))
			},
			expectedError: "Docker installation failed",
			expectedStates: map[int]models.ServiceState{
				0: models.ServiceStateFailed,
			},
		},
		{
			name: "Fail after Bacalhau install",
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Times(2).Return(nil)
				m.On("ExecuteCommand", mock.Anything).Return("", nil).Once()
				m.On("ExecuteCommand", mock.Anything).
					Return("", fmt.Errorf("Bacalhau installation failed"))
			},
			expectedError: "Bacalhau installation failed",
			expectedStates: map[int]models.ServiceState{
				0: models.ServiceStateFailed,
			},
		},
		{
			name: "Service fails to start",
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Times(3).Return(nil)
				m.On("ExecuteCommand", mock.Anything).Return("", nil).Times(2)
				m.On("ExecuteCommand", mock.Anything).
					Return("", fmt.Errorf("Service failed to start"))
			},
			expectedError: "Service failed to start",
			expectedStates: map[int]models.ServiceState{
				0: models.ServiceStateFailed,
			},
		},
	}

	for _, tc := range tests {
		tc.machines = []*models.Machine{
			{
				Orchestrator: true,
			},
		}
		for i := range tc.machines {
			for _, service := range tc.mockServices {
				tc.machines[i].SetServiceState(service.Name, service.State)
			}
		}
		runTest(t, tc, func(p *AzureProvider, ctx context.Context) error {
			return p.DeployBacalhauOrchestrator(ctx)
		})
	}
}

func TestDeployBacalhauWorkers(t *testing.T) {
	tests := []TestCase{
		{
			name: "Successful deployment",
			machines: []*models.Machine{
				{Orchestrator: true},
				{
					VMSize: "Standard_D2s_v3",
				},
				{
					VMSize: "Standard_D2s_v3",
				},
			},
			mockServices: []models.ServiceType{
				{Name: "Bacalhau", State: models.ServiceStateUnknown},
			},
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).
					Return(nil).Times(4)
				m.On("ExecuteCommand", mock.Anything).Return("", nil).Times(2)
				m.On("ExecuteCommand", "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node1"}]`, nil).Once()
				m.On("ExecuteCommand", mock.Anything).Return("", nil).Times(2)
				m.On("ExecuteCommand", "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node2"}]`, nil).Once()
			},
			expectedStates: map[int]models.ServiceState{
				1: models.ServiceStateSucceeded,
				2: models.ServiceStateSucceeded,
			},
		},
		{
			name: "Failure in script execution",
			machines: []*models.Machine{
				{Orchestrator: true},
				{
					VMSize: "Standard_D2s_v3",
				},
			},
			mockServices: []models.ServiceType{
				{Name: "Bacalhau", State: models.ServiceStateUnknown},
			},
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Return(nil)
				m.On("ExecuteCommand", mock.Anything).Return("", assert.AnError)
			},
			expectedError: "assert.AnError general error for testing",
			expectedStates: map[int]models.ServiceState{
				1: models.ServiceStateFailed,
			},
		},
		{
			name: "Invalid JSON in Bacalhau node list",
			machines: []*models.Machine{
				{Orchestrator: true},
				{
					VMSize: "Standard_D2s_v3",
				},
			},
			mockServices: []models.ServiceType{
				{Name: "Bacalhau", State: models.ServiceStateUnknown},
			},
			mockSetup: func(m *mockSSHConfig) {
				m.On("PushFile", mock.Anything, mock.Anything, true).Return(nil)
				m.On("ExecuteCommand", mock.Anything).Return("", nil)
				m.On("ExecuteCommand", "bacalhau node list --output json --api-host 0.0.0.0").
					Return("invalid json", nil)
			},
			expectedError: "failed to unmarshal node list output",
			expectedStates: map[int]models.ServiceState{
				1: models.ServiceStateFailed,
			},
		},
	}

	for _, tc := range tests {
		runTest(t, tc, func(p *AzureProvider, ctx context.Context) error {
			return p.DeployBacalhauWorkers(ctx)
		})
	}
}
