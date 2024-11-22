package provision_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/cmd/beta/provision"
	"github.com/bacalhau-project/andaime/internal/testutil"
	common_mock "github.com/bacalhau-project/andaime/mocks/common"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CmdBetaProvisionTestSuite struct {
	suite.Suite
	testSSHPublicKeyPath  string
	testSSHPrivateKeyPath string
	cleanupPublicKey      func()
	cleanupPrivateKey     func()
	tmpDir                string
	mockSSHConfig         *sshutils.MockSSHConfig
	mockClusterDeployer   *common_mock.MockClusterDeployerer
	origNewSSHConfigFunc  func(string, int, string, string) (sshutils.SSHConfiger, error)
	testLogger            *logger.TestLogger // Add testLogger field
}

func (cbpts *CmdBetaProvisionTestSuite) SetupSuite() {
	cbpts.testSSHPublicKeyPath, cbpts.cleanupPublicKey,
		cbpts.testSSHPrivateKeyPath, cbpts.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	cbpts.origNewSSHConfigFunc = sshutils.NewSSHConfigFunc
}

func (cbpts *CmdBetaProvisionTestSuite) TearDownSuite() {
	cbpts.cleanupPublicKey()
	cbpts.cleanupPrivateKey()
	sshutils.NewSSHConfigFunc = cbpts.origNewSSHConfigFunc
}

func (cbpts *CmdBetaProvisionTestSuite) SetupTest() {
	cbpts.tmpDir = cbpts.T().TempDir()

	// Initialize test logger and store it
	cbpts.testLogger = logger.NewTestLogger(cbpts.T())
	logger.SetGlobalLogger(cbpts.testLogger)

	// Create new mocks for each test
	cbpts.mockSSHConfig = new(sshutils.MockSSHConfig)
	cbpts.mockClusterDeployer = new(common_mock.MockClusterDeployerer)

	// Set up the mock SSH config function
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return cbpts.mockSSHConfig, nil
	}

	// Set up default expectations with .Maybe() to make them optional
	cbpts.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()
	cbpts.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil).Maybe()
	cbpts.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()
	cbpts.mockSSHConfig.On("InstallSystemdService",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
	cbpts.mockSSHConfig.On("RestartService",
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
	cbpts.mockSSHConfig.On("InstallBacalhau",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
	cbpts.mockSSHConfig.On("InstallDocker",
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()

	cbpts.mockClusterDeployer.On("ProvisionBacalhauNode",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
}

func (cbpts *CmdBetaProvisionTestSuite) TearDownTest() {
	// Verify all mock expectations were met
	cbpts.mockClusterDeployer.AssertExpectations(cbpts.T())
}

func (cbpts *CmdBetaProvisionTestSuite) TestNewProvisioner() {
	tests := []struct {
		name        string
		config      *provision.NodeConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &provision.NodeConfig{
				IPAddress:  "192.168.1.1",
				Username:   "testuser",
				PrivateKey: cbpts.testSSHPrivateKeyPath,
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "node config cannot be nil",
		},
		{
			name: "missing IP",
			config: &provision.NodeConfig{
				Username:   "testuser",
				PrivateKey: cbpts.testSSHPrivateKeyPath,
			},
			expectError: true,
			errorMsg:    "IP address is required",
		},
		{
			name: "missing username",
			config: &provision.NodeConfig{
				IPAddress:  "192.168.1.1",
				PrivateKey: cbpts.testSSHPrivateKeyPath,
			},
			expectError: true,
			errorMsg:    "username is required",
		},
		{
			name: "missing private key",
			config: &provision.NodeConfig{
				IPAddress: "192.168.1.1",
				Username:  "testuser",
			},
			expectError: true,
			errorMsg:    "private key is required",
		},
	}

	for _, tt := range tests {
		cbpts.Run(tt.name, func() {
			p, err := provision.NewProvisioner(tt.config)
			if tt.expectError {
				cbpts.Error(err)
				cbpts.Contains(err.Error(), tt.errorMsg)
			} else {
				cbpts.NoError(err)
				cbpts.NotNil(p)
			}
		})
	}
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvision() {
	config := &provision.NodeConfig{
		IPAddress:  "192.168.1.1",
		Username:   "testuser",
		PrivateKey: cbpts.testSSHPrivateKeyPath,
	}

	originalCalls := cbpts.mockSSHConfig.ExpectedCalls
	// Clear existing ExecuteCommand expectations
	cbpts.mockSSHConfig.ExpectedCalls = nil

	// Add our specific expectation first
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo docker run hello-world",
	).Return("Hello from Docker!", nil).Once()
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 0.0.0.0",
	).Return(`[{"id":"1234567890"}]`, nil).Once()
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo bacalhau config list --output json",
	).Return(`[]`, nil).Once()

	cbpts.mockSSHConfig.ExpectedCalls = append(cbpts.mockSSHConfig.ExpectedCalls,
		originalCalls...)

	p, err := provision.NewProvisioner(config)
	cbpts.Require().NoError(err)
	p.SetClusterDeployer(cbpts.mockClusterDeployer)

	err = p.Provision(context.Background())
	cbpts.NoError(err)
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvisionWithSettings() {
	settingsContent := `
setting.one: value1
setting.two: "value2"
`
	settingsFile := filepath.Join(cbpts.tmpDir, "settings.conf")
	err := os.WriteFile(settingsFile, []byte(settingsContent), 0644)
	cbpts.Require().NoError(err)

	config := &provision.NodeConfig{
		IPAddress:            "192.168.1.1",
		Username:             "testuser",
		PrivateKey:           cbpts.testSSHPrivateKeyPath,
		BacalhauSettingsPath: settingsFile,
	}

	originalCalls := cbpts.mockSSHConfig.ExpectedCalls
	// Clear existing ExecuteCommand expectations
	cbpts.mockSSHConfig.ExpectedCalls = nil

	// Add our specific expectation first
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo docker run hello-world",
	).Return("Hello from Docker!", nil).Once()
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 0.0.0.0",
	).Return(`[{"id":"1234567890"}]`, nil).Once()
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo bacalhau config list --output json",
	).Return(`[]`, nil).Once()

	cbpts.mockSSHConfig.ExpectedCalls = append(cbpts.mockSSHConfig.ExpectedCalls,
		originalCalls...)

	p, err := provision.NewProvisioner(config)
	cbpts.Require().NoError(err)
	p.SetClusterDeployer(cbpts.mockClusterDeployer)

	err = p.Provision(context.Background())
	cbpts.NoError(err)
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvisionWithInvalidSettings() {
	settingsContent := `
invalid-setting
`
	settingsFile := filepath.Join(cbpts.tmpDir, "settings.conf")
	err := os.WriteFile(settingsFile, []byte(settingsContent), 0644)
	cbpts.Require().NoError(err)

	config := &provision.NodeConfig{
		IPAddress:            "192.168.1.1",
		Username:             "testuser",
		PrivateKey:           cbpts.testSSHPrivateKeyPath,
		BacalhauSettingsPath: settingsFile,
	}

	p, err := provision.NewProvisioner(config)
	cbpts.Require().NoError(err)
	p.SetClusterDeployer(cbpts.mockClusterDeployer)

	err = p.Provision(context.Background())
	cbpts.Error(err)
	cbpts.Contains(err.Error(), "invalid format")
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvisionWithDockerCheck() {
	config := &provision.NodeConfig{
		IPAddress:  "192.168.1.1",
		Username:   "testuser",
		PrivateKey: cbpts.testSSHPrivateKeyPath,
	}

	originalCalls := cbpts.mockSSHConfig.ExpectedCalls
	// Clear existing ExecuteCommand expectations
	cbpts.mockSSHConfig.ExpectedCalls = nil

	// Add our specific expectation first
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo docker run hello-world",
	).Return("Hello from Docker!", nil).Once()
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 0.0.0.0",
	).Return(`[{"id":"1234567890"}]`, nil).Once()
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo bacalhau config list --output json",
	).Return(`[]`, nil).Once()

	cbpts.mockSSHConfig.ExpectedCalls = append(cbpts.mockSSHConfig.ExpectedCalls,
		originalCalls...)

	p, err := provision.NewProvisioner(config)
	cbpts.Require().NoError(err)
	p.SetClusterDeployer(cbpts.mockClusterDeployer)

	err = p.Provision(context.Background())
	cbpts.NoError(err)
}

type MockSSHConfig struct {
	mock.Mock
}

func (m *MockSSHConfig) WaitForSSH(ctx context.Context, retries int, timeout time.Duration) error {
	args := m.Called(ctx, retries, timeout)
	return args.Error(0)
}

func (m *MockSSHConfig) ExecuteCommand(cmd string) (string, error) {
	args := m.Called(cmd)
	return args.String(0), args.Error(1)
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvisionerLowLevelFailure() {
	// Setup test logger to capture output
	logCapture := logger.NewTestLogger(cbpts.T())
	logger.SetGlobalLogger(logCapture)

	// Create a mock SSH config
	mockSSH := new(sshutils.MockSSHConfig)

	// Setup the mock to pass SSH wait but fail command execution
	mockSSH.On("WaitForSSH", mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	mockSSH.On("PushFile",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	mockSSH.On("ExecuteCommand", mock.Anything, mock.Anything).Return(
		"Permission denied: cannot execute command",
		fmt.Errorf("command failed: permission denied"),
	)

	// Create a provisioner with test configuration
	config := &provision.NodeConfig{
		IPAddress:  "192.168.1.100",
		Username:   "testuser",
		PrivateKey: "/path/to/key",
	}

	testMachine, err := models.NewMachine(
		models.DeploymentTypeAWS,
		"us-east-1",
		"test",
		1,
		models.CloudSpecificInfo{},
	)
	testMachine.SetNodeType(models.BacalhauNodeTypeOrchestrator)

	cbpts.Require().NoError(err)

	p := &provision.Provisioner{
		SSHConfig: mockSSH,
		Config:    config,
		Machine:   testMachine,
	}

	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSH, nil
	}

	// Execute provisioning
	err = p.Provision(context.Background())

	// Verify error is returned
	cbpts.Error(err)
	errString := err.Error()
	cbpts.Contains(errString, "permission denied")

	// Print captured logs for debugging
	logCapture.PrintLogs(cbpts.T())

	// Verify all mock expectations were met
	mockSSH.AssertExpectations(cbpts.T())
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvisionerSimulation() {
	// Create a temporary SSH key file for testing
	tmpKeyFile := filepath.Join(cbpts.tmpDir, "test_ssh_key")
	err := os.WriteFile(tmpKeyFile, []byte("test-key-content"), 0600)
	cbpts.Require().NoError(err)

	config := &provision.NodeConfig{
		IPAddress:  "192.168.1.100",
		Username:   "test-user",
		PrivateKey: tmpKeyFile,
	}

	p, err := provision.NewProvisioner(config)
	cbpts.Require().NoError(err, "Failed to create provisioner")

	// Enable test mode
	testMode = true
	defer func() { testMode = false }()

	// Run the test mode simulation
	err = p.Provision(context.Background())
	cbpts.NoError(err)
}

func TestProvisionerSuite(t *testing.T) {
	suite.Run(t, new(CmdBetaProvisionTestSuite))
}
