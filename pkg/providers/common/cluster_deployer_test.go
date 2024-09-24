package common

import (
	"context"
	"errors"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
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

// MockMachine and its methods remain unchanged
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
