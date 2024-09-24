package common

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
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
			{Cmd: mock.Anything, Error: nil, Times: 3},
			{
				CmdMatcher: func(cmd string) bool { return true },
				Output:     `[{"name":"orch","address":"1.1.1.1"}]`,
				Error:      nil,
				Times:      1,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{Error: nil, Times: 1},
		RestartServiceExpectation:        &sshutils.Expectation{Error: nil, Times: 1},
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
		expectedOutput   string
		expectedError    string
	}{
		{
			name:             "Successful script execution",
			customScriptPath: "/path/to/valid/script.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "bash /path/to/valid/script.sh",
						Output: "Script output",
						Error:  nil,
					},
				},
			},
			expectedOutput: "Script output",
			expectedError:  "",
		},
		{
			name:             "Script execution failure",
			customScriptPath: "/path/to/valid/script.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:   "bash /path/to/valid/script.sh",
						Error: &ssh.ExitError{Waitmsg: ssh.Waitmsg{ExitStatus(): 1}},
					},
				},
			},
			expectedOutput: "",
			expectedError:  "custom script execution failed with exit code 1",
		},
		{
			name:             "Script execution timeout",
			customScriptPath: "/path/to/valid/script.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:   "bash /path/to/valid/script.sh",
						Error: context.DeadlineExceeded,
					},
				},
			},
			expectedOutput: "",
			expectedError:  "custom script execution timed out",
		},
		{
			name:             "Empty custom script path",
			customScriptPath: "",
			sshBehavior:      sshutils.ExpectedSSHBehavior{},
			expectedOutput:   "",
			expectedError:    "custom script path is empty",
		},
		{
			name:             "Non-existent custom script",
			customScriptPath: "/path/to/non-existent/script.sh",
			sshBehavior:      sshutils.ExpectedSSHBehavior{},
			expectedOutput:   "",
			expectedError:    "invalid custom script: custom script does not exist: /path/to/non-existent/script.sh",
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

			mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior)

			deployment := &models.Deployment{
				SSHUser:           "testuser",
				SSHPublicKeyPath:  "/path/to/public/key",
				SSHPrivateKeyPath: "/path/to/private/key",
				SSHPort:           22,
			}

			mockMachine := &MockMachine{
				Machine: &models.Machine{
					Name:     "test-machine",
					PublicIP: "1.2.3.4",
				},
			}

			cd := &ClusterDeployer{}

			err := cd.ExecuteCustomScript(
				context.Background(),
				mockSSHConfig,
				mockMachine,
				tt.customScriptPath,
			)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedOutput != "" {
				assert.Equal(t, tt.expectedOutput, mockSSHConfig.LastOutput)
			}
		})
	}
}
