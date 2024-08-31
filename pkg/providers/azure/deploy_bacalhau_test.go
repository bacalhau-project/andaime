package azure

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSSHConfig is a mock implementation of sshutils.SSHConfiger
type MockSSHConfig struct {
	mock.Mock
	Name        string
	MockClient  *sshutils.MockSSHClient
	MockSession *sshutils.MockSSHSession
}

func (m *MockSSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	args := m.Called(ctx, remotePath, content, executable)
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
func (m *MockSSHConfig) Connect() (sshutils.SSHClienter, error) { return m.MockClient, nil }

func (m *MockSSHConfig) Close() error { return nil }

func (m *MockSSHConfig) WaitForSSH(
	ctx context.Context,
	retries int,
	retryDelay time.Duration,
) error {
	args := m.Called(ctx, retries, retryDelay)
	return args.Error(0)
}
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
	mockSSH.MockClient = new(sshutils.MockSSHClient)

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

	viper.Set("general.project_id", "test-project")
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
	viper.Set("general.project_id", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()

	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test", VMSize: "Standard_DS4_v2", Location: "eastus2"},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("PushFile", ctx, "/tmp/get-node-config-metadata.sh", mock.Anything, true).Return(nil)
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
	viper.Set("general.project_id", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test", Orchestrator: true},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("PushFile", ctx, "/tmp/install-bacalhau.sh", mock.Anything, true).
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
	viper.Set("general.project_id", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.Machines = map[string]*models.Machine{
		"test": {Name: "test", Orchestrator: true},
	}
	deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

	mockSSH.On("PushFile", ctx, "/tmp/install-run-bacalhau.sh", mock.Anything, true).
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
	viper.Set("general.project_id", "test-project")
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
			expectedError:  "failed to strip and parse JSON",
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
		machines      map[string]*models.Machine
		expectedError string
	}{
		{
			name:     "Successful orchestrator deployment on bacalhau node",
			nodeType: "requester",
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node1"}]`, nil)
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(3)
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(3)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)
			},
			machines: map[string]*models.Machine{
				"test": {Name: "test", Orchestrator: true, PublicIP: "1.2.3.4"},
			},
			expectedError: "",
		},
		{
			name:     "Failed orchestrator deployment",
			nodeType: "requester",
			machines: map[string]*models.Machine{
				"test": {Name: "test", Orchestrator: true, PublicIP: "1.2.3.4"},
			},
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
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 1.2.3.4").
					Return(`[{"id": "node1"}]`, nil)
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(3)
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(3)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)
			},
			machines: map[string]*models.Machine{
				"test": {
					Name:           "test",
					Orchestrator:   false,
					PublicIP:       "2.3.4.5",
					OrchestratorIP: "1.2.3.4",
				},
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

			err := deployer.deployBacalhauNode(
				context.Background(),
				"test",
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
	hostname := "orch"
	ip := "1.2.3.4"
	location := "eastus"
	orchestrators := "0.0.0.0"
	vmSize := "Standard_DS4_v2"

	expectedLines := map[string][]string{
		"get-node-config-metadata.sh": {
			fmt.Sprintf(`MACHINE_TYPE="%s"`, vmSize),
			fmt.Sprintf(`HOSTNAME="%s"`, hostname),
			`VCPU_COUNT="$VCPU_COUNT"`,
			`MEMORY_GB="$MEMORY_GB"`,
			`DISK_GB="$DISK_SIZE"`,
			fmt.Sprintf(`LOCATION="%s"`, location),
			fmt.Sprintf(`IP="%s"`, ip),
			fmt.Sprintf(`ORCHESTRATORS="%s"`, orchestrators),
			`TOKEN=""`,
			`NODE_TYPE="requester"`,
		},
		"install-bacalhau.sh": {
			`sudo curl -sSL https://get.bacalhau.org/install.sh?dl="${BACALHAU_INSTALL_ID}" | sudo bash`,
		},
		"install-run-bacalhau.sh": {
			`    /usr/local/bin/bacalhau serve \`, // spacing and end line is correct here
		},
	}

	tests := []struct {
		name          string
		machines      map[string]*models.Machine
		setupMock     func(*MockSSHConfig, map[string][]byte)
		expectedError string
	}{
		{
			name: "Successful orchestrator deployment",
			machines: map[string]*models.Machine{
				"orch": {Name: hostname,
					Orchestrator: true,
					PublicIP:     ip,
					VMSize:       vmSize,
					Location:     location,
				},
			},
			setupMock: func(mockSSH *MockSSHConfig, renderedScripts map[string][]byte) {
				mockSSH.On("PushFile", ctx, "/tmp/get-node-config-metadata.sh", mock.Anything, true).
					Run(func(args mock.Arguments) {
						if args.Get(1).(string) == "/tmp/get-node-config-metadata.sh" {
							renderedScripts["get-node-config-metadata.sh"] = args.Get(2).([]byte)
						}
					}).
					Return(nil).
					Once()
				mockSSH.On("PushFile", ctx, "/tmp/install-bacalhau.sh", mock.Anything, true).
					Run(func(args mock.Arguments) {
						if args.Get(1).(string) == "/tmp/install-bacalhau.sh" {
							renderedScripts["install-bacalhau.sh"] = args.Get(2).([]byte)
						}
					}).
					Return(nil).
					Once()
				mockSSH.On("PushFile", ctx, "/tmp/install-run-bacalhau.sh", mock.Anything, true).
					Run(func(args mock.Arguments) {
						if args.Get(1).(string) == "/tmp/install-run-bacalhau.sh" {
							renderedScripts["install-run-bacalhau.sh"] = args.Get(2).([]byte)
						}
					}).
					Return(nil).
					Once()
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(3)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).Return(nil)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node1"}]`, nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := display.GetGlobalModelFunc()
			m.Deployment.Machines = tt.machines
			deployer, mockSSH := setupTestBacalhauDeployer(m.Deployment.Machines)

			renderedScripts := make(map[string][]byte)
			tt.setupMock(mockSSH, renderedScripts)

			err := deployer.DeployOrchestrator(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, models.ServiceStateSucceeded, m.Deployment.Machines["orch"].GetServiceState("Bacalhau"))
				assert.NotEmpty(t, m.Deployment.OrchestratorIP)

				// Check the content of the second PushFile call
				// Create a temp file and write the content of the second PushFile call to it
				thisFile := "get-node-config-metadata.sh"
				tempFile, err := os.CreateTemp("/tmp", thisFile)
				assert.NoError(t, err)
				_, err = tempFile.Write(renderedScripts[thisFile])
				assert.NoError(t, err)
				tempFile.Close()
				defer os.Remove(tempFile.Name())

				filesToTest := []string{"get-node-config-metadata.sh", "install-bacalhau.sh", "install-run-bacalhau.sh"}
				// Check the content of each script
				for _, fileToTest := range filesToTest {
					for _, expectedLine := range expectedLines[fileToTest] {
						if !bytes.Contains(renderedScripts[fileToTest], []byte(expectedLine+"\n")) {
							assert.Fail(t, fmt.Sprintf("Expected line not found: %s", expectedLine), fileToTest)
						}
					}
				}
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
				"orch": {Name: "orch", Orchestrator: true, PublicIP: "1.2.3.4"},
				"worker1": {
					Name:           "worker1",
					Orchestrator:   false,
					PublicIP:       "2.3.4.5",
					OrchestratorIP: "1.2.3.4",
				},
				"worker2": {
					Name:           "worker2",
					Orchestrator:   false,
					PublicIP:       "3.4.5.6",
					OrchestratorIP: "1.2.3.4",
				},
			},
			setupMock: func(mockSSH *MockSSHConfig) {
				mockSSH.On("PushFile", ctx, mock.Anything, mock.Anything, true).Return(nil).Times(9)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 1.2.3.4").
					Return(`[{"id": "node1"}]`, nil).
					Times(2)
				mockSSH.On("ExecuteCommand", ctx, "bacalhau node list --output json --api-host 0.0.0.0").
					Return(`[{"id": "node1"}]`, nil).
					Once()
				mockSSH.On("ExecuteCommand", ctx, mock.Anything).Return("", nil).Times(9)
				mockSSH.On("InstallSystemdService", ctx, "bacalhau", mock.Anything).
					Return(nil).
					Times(3)
				mockSSH.On("RestartService", ctx, "bacalhau").Return(nil).Times(3)
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

			var err error
			for _, machine := range tt.machines {
				if !machine.Orchestrator {
					err = deployer.DeployWorker(ctx, machine.Name)
					assert.NoError(t, err)
				} else {
					err = deployer.DeployOrchestrator(ctx)
					assert.NoError(t, err)
				}
				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, models.ServiceStateSucceeded, m.Deployment.Machines[machine.Name].GetServiceState("Bacalhau"))
				}
			}
			mockSSH.AssertExpectations(t)
		})
	}
}

func createMockMachine(
	name string,
) *models.Machine {
	return &models.Machine{
		Name:     name,
		VMSize:   "fake-vm-size",
		Location: "fake-location",
		SSHPort:  66000,
		SSHUser:  "fake-user",
	}
}
