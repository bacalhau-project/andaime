package provision_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bacalhau-project/andaime/cmd/beta/provision"
	"github.com/bacalhau-project/andaime/internal/testutil"
	common_mock "github.com/bacalhau-project/andaime/mocks/common"
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

func TestProvisionerSuite(t *testing.T) {
	suite.Run(t, new(CmdBetaProvisionTestSuite))
}
