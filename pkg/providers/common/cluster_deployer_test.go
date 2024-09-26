package common

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersCommonClusterDeployerTestSuite struct {
	suite.Suite
	ctx                context.Context
	clusterDeployer    *ClusterDeployer
	testPrivateKeyPath string
	cleanup            func()
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) SetupSuite() {
	s.ctx = context.Background()
	_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	s.cleanup = func() {
		cleanupPublicKey()
		cleanupPrivateKey()
	}
	viper.Set("general.project_prefix", "test-project")
	s.testPrivateKeyPath = testPrivateKeyPath
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) TearDownSuite() {
	s.cleanup()
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) SetupTest() {
	s.clusterDeployer = NewClusterDeployer(models.DeploymentTypeUnknown)
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) TestProvisionAllMachinesWithPackages() {
	tests := []struct {
		name          string
		sshBehavior   sshutils.ExpectedSSHBehavior
		expectedError string
	}{
		{
			name: "successful provisioning",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{Dst: "/tmp/install-docker.sh", Executable: true, Error: nil, Times: 1},
					{Dst: "/tmp/install-core-packages.sh", Executable: true, Error: nil, Times: 1},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{Cmd: "sudo /tmp/install-docker.sh", Error: nil, Times: 1},
					{
						Cmd:    "sudo docker run hello-world",
						Error:  nil,
						Times:  1,
						Output: "Hello from Docker!",
					},
					{Cmd: "sudo /tmp/install-core-packages.sh", Error: nil, Times: 1},
				},
			},
			expectedError: "",
		},
		{
			name: "SSH error",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/install-docker.sh",
						Executable: true,
						Error:      errors.New("failed to push file"),
						Times:      1,
					},
				},
			},
			expectedError: "failed to provision packages on all machines",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
				return sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior), nil
			}

			deployment := &models.Deployment{
				Machines: map[string]models.Machiner{
					"machine1": NewMockMachine("machine1", s.testPrivateKeyPath),
				},
			}

			m := display.GetGlobalModelFunc()
			m.Deployment = deployment

			err := s.clusterDeployer.ProvisionAllMachinesWithPackages(s.ctx)

			if tt.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedError)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) TestProvisionBacalhauCluster() {
	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{Dst: mock.Anything, Executable: true, Error: nil, Times: 3},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:   "sudo /tmp/get-node-config-metadata.sh",
				Error: nil,
				Times: 2,
			},
			{
				Cmd:   "sudo /tmp/install-bacalhau.sh",
				Error: nil,
				Times: 2,
			},
			{
				Cmd:   "sudo /tmp/install-run-bacalhau.sh",
				Error: nil,
				Times: 2,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 0.0.0.0",
				Output: `[{"name":"orch","address":"1.1.1.1"}]`,
				Error:  nil,
				Times:  1,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 1.1.1.1",
				Output: `[{"name":"orch","address":"1.1.1.1"},{"name":"worker","address":"2.2.2.2"}]`,
				Error:  nil,
				Times:  1,
			},
			{
				Cmd:    "sudo bacalhau config list --output json",
				Output: "[]",
				Error:  nil,
				Times:  2,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{Error: nil, Times: 2},
		RestartServiceExpectation:        &sshutils.Expectation{Error: nil, Times: 4},
	}

	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return sshutils.NewMockSSHConfigWithBehavior(sshBehavior), nil
	}

	m := display.GetGlobalModelFunc()
	m.Deployment = &models.Deployment{
		Machines: map[string]models.Machiner{
			"orch":   NewMockMachine("orch", s.testPrivateKeyPath),
			"worker": NewMockMachine("worker", s.testPrivateKeyPath),
		},
	}
	m.Deployment.Machines["orch"].(*MockMachine).Orchestrator = true
	m.Deployment.Machines["orch"].(*MockMachine).PublicIP = "1.1.1.1"
	m.Deployment.Machines["worker"].(*MockMachine).PublicIP = "2.2.2.2"

	err := s.clusterDeployer.ProvisionBacalhauCluster(s.ctx)

	s.NoError(err)
	s.Equal("1.1.1.1", m.Deployment.OrchestratorIP)
}

func TestPkgProvidersCommonClusterDeployerTestSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersCommonClusterDeployerTestSuite))
}

type MockMachine struct {
	mock.Mock
	*models.Machine
}

func NewMockMachine(name string, privateKeyPath string) *MockMachine {
	m := &MockMachine{
		Machine: &models.Machine{
			Name:              name,
			SSHUser:           "test-user",
			SSHPrivateKeyPath: privateKeyPath,
			SSHPort:           22,
			PublicIP:          "1.1.1.1",
			Orchestrator:      false,
		},
	}
	m.On("SetServiceState", mock.Anything, mock.Anything).Return()
	m.On("SetComplete").Return()
	return m
}

func (m *MockMachine) SetServiceState(serviceName string, state models.ServiceState) {
	m.Called(serviceName, state)
}

func (m *MockMachine) SetComplete() {
	m.Called()
}

func TestExecuteCustomScript(t *testing.T) {
	tests := []struct {
		name             string
		customScriptPath string
		sshBehavior      sshutils.ExpectedSSHBehavior
		expectedError    string
	}{
		{
			name:             "Successful script execution",
			customScriptPath: "/tmp/custom_script.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/custom_script.sh",
						Executable: true,
						Error:      nil,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
						Output: "Script output",
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name:             "Script execution failure",
			customScriptPath: "/path/to/valid/script_1.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/custom_script.sh",
						Executable: true,
						Error:      nil,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:   "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
						Error: errors.New("script execution failed"),
					},
				},
			},
			expectedError: "failed to execute custom script: script execution failed",
		},
		{
			name:             "Script execution timeout",
			customScriptPath: "/path/to/valid/script_2.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/custom_script.sh",
						Executable: true,
						Error:      nil,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:   "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
						Error: context.DeadlineExceeded,
					},
				},
			},
			expectedError: "failed to execute custom script: context deadline exceeded",
		},
		{
			name:             "Empty custom script path",
			customScriptPath: "",
			sshBehavior:      sshutils.ExpectedSSHBehavior{},
			expectedError:    "",
		},
		{
			name:             "Non-existent custom script",
			customScriptPath: "/path/to/non-existent/script.sh",
			sshBehavior:      sshutils.ExpectedSSHBehavior{},
			expectedError:    "failed to read custom script: open /path/to/non-existent/script.sh: no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file for the custom script if it's supposed to exist
			if tt.customScriptPath != "" && !strings.Contains(tt.name, "Non-existent") {
				tmpfile, err := os.CreateTemp("", "testscript")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpfile.Name())
				tt.customScriptPath = tmpfile.Name()
			}

			// Set up Viper configuration
			viper.Set("general.custom_script_path", tt.customScriptPath)
			defer viper.Reset()

			mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior)

			mockMachine := &MockMachine{
				Machine: &models.Machine{
					Name:     "test-machine",
					PublicIP: "1.2.3.4",
				},
			}
			mockMachine.On("SetServiceState", mock.Anything, mock.Anything).Return()
			mockMachine.On("SetComplete").Return()

			cd := NewClusterDeployer(models.DeploymentTypeAzure)

			err := cd.ExecuteCustomScript(
				context.Background(),
				mockSSHConfig,
				mockMachine,
			)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyBacalhauConfigs(t *testing.T) {
	type testCase struct {
		name             string
		bacalhauSettings map[string]string
		sshBehavior      sshutils.ExpectedSSHBehavior
		expectedError    string
	}

	tests := []testCase{
		{
			name:             "Empty configuration",
			bacalhauSettings: map[string]string{},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: "[]",
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Single configuration",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths": `"/tmp","/data"`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'node.allowlistedlocalpaths'","Value":["/tmp","/data"]}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Multiple configurations",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths":                 `"/tmp","/data"`,
				"orchestrator.nodemanager.disconnecttimeout": "5s",
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    `sudo bacalhau config set 'orchestrator.nodemanager.disconnecttimeout' '5s'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'node.allowlistedlocalpaths'","Value":["/tmp","/data"]},{"Key":"'orchestrator.nodemanager.disconnecttimeout'","Value":"5s"}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Configuration with error",
			bacalhauSettings: map[string]string{
				"invalid.config": "value",
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'invalid.config' 'value'`,
						Output: "",
						Error:  errors.New("invalid configuration key"),
					},
				},
			},
			expectedError: "invalid configuration key",
		},
		{
			name: "Unexpected value in configuration",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths": `"/tmp","/data"`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'node.allowlistedlocalpaths'","Value":["/tmp","/data"]}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Missing configuration in final list",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths": `"/tmp","/data"`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up Viper configuration
			viper.Set("general.bacalhau_settings", tt.bacalhauSettings)
			defer viper.Reset()

			bacalhauSettings, err := utils.ReadBacalhauSettingsFromViper()
			assert.NoError(t, err)

			deployment := &models.Deployment{
				BacalhauSettings: bacalhauSettings,
			}

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}

			mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior)

			cd := NewClusterDeployer(models.DeploymentTypeAzure)

			err = cd.ApplyBacalhauConfigs(context.Background(), mockSSHConfig)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify that all expected commands were executed
			mockSSHConfig.AssertExpectations(t)
		})
	}
}
