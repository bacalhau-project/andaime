package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/stretchr/testify/assert"
)

// mockGetGlobalModel replaces the global GetGlobalModel function with a mock
func mockGetGlobalModel(mockModel *display.DisplayModel) func() {
	original := display.GetGlobalModel
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return mockModel
	}
	return func() {
		display.GetGlobalModelFunc = original
	}
}

func TestDeployBacalhauOrchestrator(t *testing.T) {
	tests := []struct {
		name          string
		machines      []models.Machine
		expectError   bool
		errorContains string
	}{
		{
			name:          "No orchestrator node and no orchestrator IP",
			machines:      []models.Machine{},
			expectError:   true,
			errorContains: "no orchestrator node found",
		},
		{
			name: "No orchestrator node, but orchestrator IP specified",
			machines: []models.Machine{
				{OrchestratorIP: "NOT_EMPTY"},
			},
			expectError:   true,
			errorContains: "orchestrator IP set",
		},
		{
			name: "One orchestrator node, no other machines",
			machines: []models.Machine{
				{Orchestrator: true},
			},
			expectError: false,
		},
		{
			name: "One orchestrator node and many other machines",
			machines: []models.Machine{
				{Orchestrator: true},
				{},
				{},
			},
			expectError: false,
		},
		{
			name: "Multiple orchestrator nodes (should error)",
			machines: []models.Machine{
				{Orchestrator: true},
				{Orchestrator: true},
			},
			expectError:   true,
			errorContains: "multiple orchestrator nodes found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock model with the test machines
			mockModel := &display.DisplayModel{
				Deployment: &models.Deployment{
					Machines: tt.machines,
				},
			}

			// Replace the global GetGlobalModel with our mock
			cleanup := mockGetGlobalModel(mockModel)
			defer cleanup()

			// Create an AzureProvider instance
			provider := &AzureProvider{}

			// Call the method
			err := provider.DeployBacalhauOrchestrator(context.Background())

			// Check the result
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type mockSSHConfig struct {
	pushFileOutputs           []string
	executeCommandOutputs     []string
	pushFileErrs              []error
	executeCommandErrs        []error
	currentPushFileCall       int
	currentExecuteCommandCall int
}

func (m *mockSSHConfig) Connect() (sshutils.SSHClienter, error) {
	return nil, nil
}

func (m *mockSSHConfig) Close() error {
	return nil
}

func (m *mockSSHConfig) PushFile(content []byte, remotePath string, executable bool) error {
	if m.currentPushFileCall >= len(m.pushFileErrs) {
		return nil
	}
	err := m.pushFileErrs[m.currentPushFileCall]
	m.currentPushFileCall++
	return err
}

func (m *mockSSHConfig) ExecuteCommand(cmd string) (string, error) {
	if m.currentExecuteCommandCall >= len(m.executeCommandOutputs) {
		return "", nil
	}
	output := m.executeCommandOutputs[m.currentExecuteCommandCall]
	err := m.executeCommandErrs[m.currentExecuteCommandCall]
	m.currentExecuteCommandCall++
	return output, err
}

func (m *mockSSHConfig) InstallSystemdService(serviceName string, servicePath string) error {
	return nil
}

func (m *mockSSHConfig) RestartService(serviceName string) error {
	return nil
}

func (m *mockSSHConfig) SetSSHClient(client sshutils.SSHClienter) {
	return
}

func (m *mockSSHConfig) StartService(serviceName string) error {
	return nil
}

func TestDeployBacalhauOrchestrator_FailureScenarios(t *testing.T) {
	tests := []struct {
		name                  string
		pushFileOutputs       []string
		executeCommandOutputs []string
		pushFileErrs          []error
		executeCommandErrs    []error
		setupMock             func() sshutils.SSHConfiger
		expectedError         string
		expectedState         models.ServiceState
	}{
		{
			name:                  "Fail after Docker install",
			pushFileOutputs:       []string{"success"},
			executeCommandOutputs: []string{"success"},
			pushFileErrs:          []error{nil},
			executeCommandErrs:    []error{fmt.Errorf("Docker installation failed")},
			setupMock: func() sshutils.SSHConfiger {
				return &mockSSHConfig{
					pushFileOutputs:       []string{"success"},
					executeCommandOutputs: []string{"success"},
					pushFileErrs:          []error{nil},
					executeCommandErrs:    []error{fmt.Errorf("Docker installation failed")},
				}
			},
			expectedError: "Docker installation failed",
			expectedState: models.ServiceStateFailed,
		},
		{
			name:                  "Fail after Bacalhau install",
			pushFileOutputs:       []string{"success"},
			executeCommandOutputs: []string{"success"},
			pushFileErrs:          []error{nil},
			executeCommandErrs:    []error{fmt.Errorf("Bacalhau installation failed")},
			setupMock: func() sshutils.SSHConfiger {
				return &mockSSHConfig{
					pushFileOutputs:       []string{"success"},
					executeCommandOutputs: []string{"success"},
					pushFileErrs:          []error{nil},
					executeCommandErrs:    []error{fmt.Errorf("Bacalhau installation failed")},
				}
			},
			expectedError: "Bacalhau installation failed",
			expectedState: models.ServiceStateFailed,
		},
		{
			name:                  "Fail after service install",
			pushFileOutputs:       []string{"success", "success"},
			executeCommandOutputs: []string{"success", "success"},
			pushFileErrs:          []error{nil, nil},
			executeCommandErrs:    []error{nil, fmt.Errorf("Service installation failed")},
			setupMock: func() sshutils.SSHConfiger {
				return &mockSSHConfig{
					pushFileOutputs:       []string{"success"},
					executeCommandOutputs: []string{"success"},
					pushFileErrs:          []error{nil},
					executeCommandErrs:    []error{fmt.Errorf("Service installation failed")},
				}
			},
			expectedError: "Service installation failed",
			expectedState: models.ServiceStateFailed,
		},
		{
			name:                  "Service fails to start",
			pushFileOutputs:       []string{"success"},
			executeCommandOutputs: []string{"failure"},
			pushFileErrs:          []error{nil},
			executeCommandErrs:    []error{fmt.Errorf("Service failed to start")},
			setupMock: func() sshutils.SSHConfiger {
				return &mockSSHConfig{
					pushFileOutputs:       []string{"success"},
					executeCommandOutputs: []string{"failure"},
					pushFileErrs:          []error{nil},
					executeCommandErrs:    []error{fmt.Errorf("Service failed to start")},
				}
			},
			expectedError: "Service failed to start",
			expectedState: models.ServiceStateFailed,
		},
		{
			name:                  "Service fails to respond on version",
			pushFileOutputs:       []string{"success"},
			executeCommandOutputs: []string{"failure"},
			pushFileErrs:          []error{nil},
			executeCommandErrs:    []error{fmt.Errorf("Service did not respond with version")},
			setupMock: func() sshutils.SSHConfiger {
				return &mockSSHConfig{
					pushFileOutputs:       []string{"success"},
					executeCommandOutputs: []string{"failure"},
					pushFileErrs:          []error{nil},
					executeCommandErrs: []error{
						fmt.Errorf("Service did not respond with version"),
					},
				}
			},
			expectedError: "Service did not respond with version",
			expectedState: models.ServiceStateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the mock SSHConfig
			mockSSH := tt.setupMock()

			// Mock the NewSSHConfig function to return our mockSSHConfig
			originalNewSSHConfig := sshutils.NewSSHConfig
			sshutils.NewSSHConfigFunc = func(host string, port int, user string, privateKeyMaterial []byte) (sshutils.SSHConfiger, error) {
				return mockSSH, nil
			}
			defer func() { sshutils.NewSSHConfigFunc = originalNewSSHConfig }() // Restore the original function

			// Replace the global GetGlobalModel function with a mock
			mockModel := &display.DisplayModel{
				Deployment: &models.Deployment{
					Machines: []models.Machine{
						{
							Orchestrator: true,
							MachineServices: map[string]models.ServiceType{
								"Bacalhau": {State: models.ServiceStateUnknown},
							},
						},
					},
				},
			}
			cleanup := mockGetGlobalModel(mockModel)
			defer cleanup()

			provider := &AzureProvider{}

			// Call the function
			err := provider.DeployBacalhauOrchestrator(context.Background())

			// Check that the error matches the expected error
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify the state of the service
			assert.Equal(
				t,
				tt.expectedState,
				mockModel.Deployment.Machines[0].MachineServices["Bacalhau"].State,
			)
		})
	}
}
