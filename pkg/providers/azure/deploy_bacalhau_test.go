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

// MockSSHConfig is a mock implementation of sshutils.SSHConfiger
type MockSSHConfig struct {
	mock.Mock
}

func (m *MockSSHConfig) PushFile(
	ctx context.Context,
	content []byte,
	remotePath string,
	executable bool,
) error {
	args := m.Called(ctx, content, remotePath, executable)
	return args.Error(0)
}

func (m *MockSSHConfig) ExecuteCommand(
	ctx context.Context,
	cmd string,
) (string, error) {
	args := m.Called(ctx, cmd)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfig) InstallSystemdService(
	ctx context.Context,
	serviceName string,
	serviceContent string,
) error {
	args := m.Called(ctx, serviceName, serviceContent)
	return args.Error(0)
}

func (m *MockSSHConfig) RestartService(
	ctx context.Context,
	serviceName string,
) error {
	args := m.Called(ctx, serviceName)
	return args.Error(0)
}

// Additional methods to satisfy the SSHConfiger interface
func (m *MockSSHConfig) Connect() (sshutils.SSHClienter, error) { return nil, nil }

func (m *MockSSHConfig) Close() error                             { return nil }
func (m *MockSSHConfig) SetSSHClient(client sshutils.SSHClienter) {}

func (m *MockSSHConfig) StartService(
	ctx context.Context,
	serviceName string,
) error {
	return nil
}

func setupTestBacalhauDeployer(
	machines map[string]*models.Machine,
) (*BacalhauDeployer, *MockSSHConfig) {
	mockSSH := new(MockSSHConfig)

	m := display.GetGlobalModelFunc()
	m.Deployment.Machines = machines
	m.Deployment.OrchestratorIP = "1.2.3.4"

	deployer := NewBacalhauDeployer()

	// Override the SSH config creation function
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		privateKeyMaterial []byte,
	) (sshutils.SSHConfiger, error) {
		return mockSSH, nil
	}

	return deployer, mockSSH
}

func TestFindOrchestratorMachine(t *testing.T) {
	tests := []struct {
		name          string
		machines      map[string]*models.Machine
		expectedError string
	}{
		{
			name: "Single orchestrator",
			machines: map[string]*models.Machine{
				"orch":   {Name: "orch", Orchestrator: true},
				"worker": {Name: "worker"},
			},
			expectedError: "",
		},
		{
			name: "No orchestrator",
			machines: map[string]*models.Machine{
				"worker1": {Name: "worker1"},
				"worker2": {Name: "worker2"},
			},
			expectedError: "no orchestrator node found",
		},
		{
			name: "Multiple orchestrators",
			machines: map[string]*models.Machine{
				"orch1": {Name: "orch1", Orchestrator: true},
				"orch2": {Name: "orch2", Orchestrator: true},
			},
			expectedError: "multiple orchestrator nodes found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployer, _ := setupTestBacalhauDeployer(tt.machines)
			machine, err := deployer.findOrchestratorMachine()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, machine)
				assert.True(t, machine.Orchestrator)
			}
		})
	}
}

func TestSetupNodeConfigMetadata(t *testing.T) {
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test", VMSize: "Standard_D2s_v3", Location: "eastus"},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("PushFile", ctx, mock.Anything, "/tmp/get-node-config-metadata.sh", true).Return(nil)
	mockSSH.On("ExecuteCommand", ctx, "sudo /tmp/get-node-config-metadata.sh").Return("", nil)

	err := deployer.setupNodeConfigMetadata(
		ctx,
		m.Deployment.Machines["test"],
		mockSSH,
		"compute",
	)

	assert.NoError(t, err)
	mockSSH.AssertExpectations(t)
}

func TestInstallBacalhau(t *testing.T) {
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test", Orchestrator: true},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("PushFile", ctx, mock.Anything, "/tmp/install-bacalhau.sh", true).
		Return(nil).
		Times(1)
	mockSSH.On("ExecuteCommand", ctx, "sudo /tmp/install-bacalhau.sh").Return("", nil)

	err := deployer.installBacalhau(
		ctx,
		mockSSH,
	)

	assert.NoError(t, err)
	mockSSH.AssertExpectations(t)
}

func TestInstallBacalhauRun(t *testing.T) {
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test", Orchestrator: true},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("PushFile", ctx, mock.Anything, "/tmp/install-run-bacalhau.sh", true).
		Return(nil).
		Times(1)
	mockSSH.On("ExecuteCommand", ctx, "sudo /tmp/install-run-bacalhau.sh").Return("", nil)

	err := deployer.installBacalhauRunScript(
		ctx,
		mockSSH,
	)

	assert.NoError(t, err)
	mockSSH.AssertExpectations(t)
}

func TestSetupBacalhauService(t *testing.T) {
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test"},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
	mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)

	err := deployer.setupBacalhauService(
		ctx,
		mockSSH,
	)

	assert.NoError(t, err)
	mockSSH.AssertExpectations(t)
}

func TestVerifyBacalhauDeployment(t *testing.T) {
	tests := []struct {
		name           string
		nodeListOutput string
		expectedError  string
	}{
		{
			name:           "Successful verification",
			nodeListOutput: `[{"id": "node1"}]`,
			expectedError:  "",
		},
		{
			name:           "Empty node list",
			nodeListOutput: `[]`,
			expectedError:  "no Bacalhau nodes found in the output",
		},
		{
			name:           "Invalid JSON",
			nodeListOutput: `invalid json`,
			expectedError:  "failed to unmarshal node list output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			deployer, mockSSH := setupTestBacalhauDeployer(map[string]*models.Machine{
				"test": {Name: "test"},
			})

			mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 0.0.0.0").
				Return(tt.nodeListOutput, nil)

			err := deployer.verifyBacalhauDeployment(
				ctx,
				mockSSH,
				"0.0.0.0",
			)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockSSH.AssertExpectations(t)
		})
	}
}

func TestDeployBacalhauNode(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		nodeType      string
		setupMock     func(*MockSSHConfig)
		expectedError string
	}{
		{
			name:     "Successful orchestrator deployment on bacalhau node",
			nodeType: "requester",
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(3)
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(3)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 1.2.3.4").
					Return(`[{"id": "node1"}]`, nil)
			},
			expectedError: "",
		},
		{
			name:     "Failed orchestrator deployment",
			nodeType: "requester",
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).
					Return(fmt.Errorf("push file error"))
			},
			expectedError: "failed to push node config metadata script",
		},
		{
			name:     "Successful worker deployment",
			nodeType: "compute",
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(3)
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(3)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 1.2.3.4").
					Return(`[{"id": "node1"}]`, nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := display.GetGlobalModelFunc()
			m.Deployment.Machines = map[string]*models.Machine{
				"test": {Name: "test", Orchestrator: tt.nodeType == "requester"},
			}
			deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

			tt.setupMock(mockSSH)

			err := deployer.deployBacalhauNode(
				context.Background(),
				m.Deployment.Machines["test"],
				tt.nodeType,
			)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, models.ServiceStateSucceeded, m.Deployment.Machines["test"].GetServiceState("Bacalhau"))
			}

			mockSSH.AssertExpectations(t)
		})
	}
}

func TestDeployOrchestrator(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		machines      map[string]*models.Machine
		setupMock     func(*MockSSHConfig)
		expectedError string
	}{
		{
			name: "Successful orchestrator deployment",
			machines: map[string]*models.Machine{
				"orch": {Name: "orch", Orchestrator: true, PublicIP: "1.2.3.4"},
			},
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(3)
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(3)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 1.2.3.4").
					Return(`[{"id": "node1"}]`, nil)
			},
			expectedError: "",
		},
		{
			name: "No orchestrator node",
			machines: map[string]*models.Machine{
				"worker": {Name: "worker"},
			},
			setupMock:     func(mockSSH *MockSSHConfig) {},
			expectedError: "no orchestrator node found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := display.GetGlobalModelFunc()
			m.Deployment.Machines = tt.machines
			deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

			tt.setupMock(mockSSH)

			err := deployer.DeployOrchestrator(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, models.ServiceStateSucceeded, m.Deployment.Machines["orch"].GetServiceState("Bacalhau"))
				assert.NotEmpty(t, m.Deployment.OrchestratorIP)
			}

			mockSSH.AssertExpectations(t)
		})
	}
}

func TestDeployWorkers(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		machines      map[string]*models.Machine
		setupMock     func(*MockSSHConfig)
		expectedError string
	}{
		{
			name: "Successful workers deployment",
			machines: map[string]*models.Machine{
				"orch":    {Name: "orch", Orchestrator: true, PublicIP: "1.2.3.4"},
				"worker1": {Name: "worker1"},
				"worker2": {Name: "worker2"},
			},
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(6)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 1.2.3.4").
					Return(`[{"id": "node1"}]`, nil).
					Times(2)
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(6)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).
					Return(nil).
					Times(2)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil).Times(2)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := display.GetGlobalModelFunc()
			m.Deployment.Machines = tt.machines
			deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

			tt.setupMock(mockSSH)

			err := deployer.DeployWorkers(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				for name, machine := range tt.machines {
					if !machine.Orchestrator {
						assert.Equal(t, models.ServiceStateSucceeded, m.Deployment.Machines[name].GetServiceState("Bacalhau"))
					}
				}
			}

			mockSSH.AssertExpectations(t)
		})
	}
}
