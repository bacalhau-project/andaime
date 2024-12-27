package provision_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bacalhau-project/andaime/cmd/beta/provision"
	"github.com/bacalhau-project/andaime/internal/testutil"
	common_mock "github.com/bacalhau-project/andaime/mocks/common"
	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
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
	mockSSHConfig         *ssh_mock.MockSSHConfiger
	mockClusterDeployer   *common_mock.MockClusterDeployerer
	origNewSSHConfigFunc  func(string, int, string, string) (sshutils_interfaces.SSHConfiger, error)
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
	cbpts.mockSSHConfig = new(ssh_mock.MockSSHConfiger)
	cbpts.mockClusterDeployer = new(common_mock.MockClusterDeployerer)

	// Create mock SSH client
	mockSSHClient := new(ssh_mock.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&ssh_mock.MockSSHSessioner{}, nil).Maybe()
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Set up the mock SSH config function
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interfaces.SSHConfiger, error) {
		return cbpts.mockSSHConfig, nil
	}

	// Set up Connect and Close expectations
	cbpts.mockSSHConfig.On("Connect").Return(mockSSHClient, nil).Maybe()
	cbpts.mockSSHConfig.On("Close").Return(nil).Maybe()

	// Set up Connect and Close expectations
	cbpts.mockSSHConfig.On("Connect").Return(mockSSHClient, nil).Maybe()
	cbpts.mockSSHConfig.On("Close").Return(nil).Maybe()

	// Set up default expectations with .Maybe() to make them optional
	cbpts.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()
	cbpts.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()

	// Set up specific Docker command expectation first
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		models.ExpectedDockerHelloWorldCommand,
	).
		Return(models.ExpectedDockerOutput, nil).
		Maybe()

	// Then set up the catch-all for other commands
	cbpts.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(cmd string) bool {
		// Exclude specific commands we want to handle separately
		return cmd != models.ExpectedDockerHelloWorldCommand &&
			!strings.Contains(cmd, "bacalhau node list") &&
			!strings.Contains(cmd, "bacalhau config list")
	})).
		Return("", nil).
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
		mock.Anything,
	).
		Return("", nil).
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
				IPAddress:      "192.168.1.1",
				Username:       "testuser",
				PrivateKeyPath: cbpts.testSSHPrivateKeyPath,
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
				Username:       "testuser",
				PrivateKeyPath: cbpts.testSSHPrivateKeyPath,
			},
			expectError: true,
			errorMsg:    "IP address is required",
		},
		{
			name: "missing username",
			config: &provision.NodeConfig{
				IPAddress:      "192.168.1.1",
				PrivateKeyPath: cbpts.testSSHPrivateKeyPath,
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
		IPAddress:      "192.168.1.1",
		Username:       "testuser",
		PrivateKeyPath: cbpts.testSSHPrivateKeyPath,
	}

	originalCalls := cbpts.mockSSHConfig.ExpectedCalls
	// Clear existing ExecuteCommand expectations
	cbpts.mockSSHConfig.ExpectedCalls = nil

	// Add our specific expectation first
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		models.ExpectedDockerHelloWorldCommand,
	).Return(models.ExpectedDockerOutput, nil).Once()
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
		PrivateKeyPath:       cbpts.testSSHPrivateKeyPath,
		BacalhauSettingsPath: settingsFile,
	}

	originalCalls := cbpts.mockSSHConfig.ExpectedCalls
	// Clear existing ExecuteCommand expectations
	cbpts.mockSSHConfig.ExpectedCalls = nil

	// Add our specific expectation first
	cbpts.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo docker run hello-world",
	).Return(models.ExpectedDockerOutput, nil).Once()
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
		PrivateKeyPath:       cbpts.testSSHPrivateKeyPath,
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
	// Create mock SSH client
	mockSSHClient := new(ssh_mock.MockSSHClienter)

	// Define expected behavior
	behavior := sshutils.ExpectedSSHBehavior{
		ConnectExpectation: &sshutils.ConnectExpectation{
			Client: mockSSHClient,
			Error:  nil,
			Times:  2,
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:    models.ExpectedDockerHelloWorldCommand,
				Times:  2,
				Output: models.ExpectedDockerOutput,
				Error:  nil,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 0.0.0.0",
				Times:  1,
				Output: `[{"id":"1234567890"}]`,
				Error:  nil,
			},
		},
		WaitForSSHCount: 1,
		WaitForSSHError: nil,
	}

	// Create mock SSH config with behavior
	mockSSH := sshutils.NewMockSSHConfigWithBehavior(behavior)

	// Set up the mock SSH config function
	origNewSSHConfigFunc := sshutils.NewSSHConfigFunc
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interfaces.SSHConfiger, error) {
		return mockSSH, nil
	}
	defer func() { sshutils.NewSSHConfigFunc = origNewSSHConfigFunc }()

	config := &provision.NodeConfig{
		IPAddress:      "192.168.1.1",
		Username:       "testuser",
		PrivateKeyPath: cbpts.testSSHPrivateKeyPath,
	}

	p, err := provision.NewProvisioner(config)
	cbpts.Require().NoError(err)

	err = p.Provision(context.Background())
	cbpts.NoError(err)

	mockSSH.(*ssh_mock.MockSSHConfiger).AssertExpectations(cbpts.T())

	mockSSH.(*ssh_mock.MockSSHConfiger).AssertExpectations(cbpts.T())
}

func (cbpts *CmdBetaProvisionTestSuite) TestProvisionerLowLevelFailure() {
	// Setup test logger to capture output
	logCapture := logger.NewTestLogger(cbpts.T())
	logger.SetGlobalLogger(logCapture)

	// Create mock SSH client
	mockSSHClient := new(ssh_mock.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&ssh_mock.MockSSHSessioner{}, nil).Maybe()
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Create a mock SSH config
	mockSSH := new(ssh_mock.MockSSHConfiger)

	// Setup the mock to pass SSH wait but fail command execution
	mockSSH.On("Connect").Return(mockSSHClient, nil).Maybe()
	mockSSH.On("Close").Return(nil).Maybe()
	mockSSH.On("Connect").Return(mockSSHClient, nil).Maybe()
	mockSSH.On("Close").Return(nil).Maybe()
	mockSSH.On("WaitForSSH", mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	mockSSH.On("ExecuteCommand",
		mock.Anything,
		"hostname",
	).Return("hostname", nil)
	mockSSH.On("ExecuteCommand", mock.Anything, mock.Anything).Return(
		"Permission denied: cannot execute command",
		fmt.Errorf("command failed: permission denied"),
	)

	// Create a provisioner with test configuration
	config := &provision.NodeConfig{
		IPAddress:      "192.168.1.100",
		Username:       "testuser",
		PrivateKeyPath: "/path/to/key",
	}

	testMachine, err := models.NewMachine(
		models.DeploymentTypeAWS,
		"us-east-1a",
		"test",
		1,
		"us-east-1",
		"us-east-1a",
		models.CloudSpecificInfo{},
	)
	testMachine.SetNodeType(models.BacalhauNodeTypeOrchestrator)

	cbpts.Require().NoError(err)

	p := &provision.Provisioner{
		SSHConfig: mockSSH,
		Config:    config,
		Machine:   testMachine,
	}

	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interfaces.SSHConfiger, error) {
		return mockSSH, nil
	}

	// Execute provisioning
	err = p.Provision(context.Background())

	// Verify error is returned
	cbpts.Error(err)
	errString := err.Error()
	cbpts.Contains(errString, "permission denied")

	// Print captured logs for debugging
	for _, log := range logCapture.GetLogs() {
		fmt.Println(log)
	}

	// Verify all mock expectations were met
	mockSSH.AssertExpectations(cbpts.T())
}

func TestProvisionerSuite(t *testing.T) {
	suite.Run(t, new(CmdBetaProvisionTestSuite))
}
