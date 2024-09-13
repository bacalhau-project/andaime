package common

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProvider implements the necessary methods for testing
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) ProvisionPackagesOnMachine(ctx context.Context, machineName string) error {
	args := m.Called(ctx, machineName)
	return args.Error(0)
}

// MockMachine implements the necessary methods for testing
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
	return m
}

func (m *MockMachine) InstallDockerAndCorePackages(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMachine) SetServiceState(serviceName string, state models.ServiceState) {
	m.Called(serviceName, state)
}

func (m *MockMachine) SetComplete() {
	m.Called()
}

func TestProvisionAllMachinesWithPackages(t *testing.T) {
	ctx := context.Background()
	_,
		cleanupPublicKey,
		testPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	viper.Set("general.project_prefix", "test-project")

	t.Run("successful provisioning", func(t *testing.T) {
		mockProvider := new(MockProvider)
		cd := NewClusterDeployer(mockProvider)

		mockSSHConfig := new(sshutils.MockSSHConfig)
		mockSSHConfig.On("PushFile", ctx, "/tmp/install-docker.sh", mock.Anything, mock.Anything).
			Return(nil)
		mockSSHConfig.On("ExecuteCommand", ctx, "sudo /tmp/install-docker.sh", mock.Anything).
			Return("", nil)
		mockSSHConfig.On("ExecuteCommand", ctx, "sudo docker run hello-world").Return("", nil)
		mockSSHConfig.On("PushFile", ctx, "/tmp/install-core-packages.sh", mock.Anything, mock.Anything).
			Return(nil)
		mockSSHConfig.On("ExecuteCommand", ctx, "sudo /tmp/install-core-packages.sh", mock.Anything).
			Return("", nil)

		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			return mockSSHConfig, nil
		}

		deployment := &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}
		deployment.Machines["machine1"] = NewMockMachine(
			"machine1",
			testPrivateKeyPath,
		)
		deployment.Machines["machine2"] = NewMockMachine(
			"machine2",
			testPrivateKeyPath,
		)
		deployment.Machines["machine3"] = NewMockMachine(
			"machine3",
			testPrivateKeyPath,
		)

		m := display.GetGlobalModelFunc()
		m.Deployment = deployment

		err := cd.ProvisionAllMachinesWithPackages(ctx)

		assert.NoError(t, err)
	})

	t.Run("error with SSH", func(t *testing.T) {
		mockProvider := new(MockProvider)
		cd := NewClusterDeployer(mockProvider)

		mockSSHConfig := new(sshutils.MockSSHConfig)
		mockSSHConfig.On("PushFile", ctx, "/tmp/install-docker.sh", mock.Anything, mock.Anything).
			Return(errors.New("failed to push file"))
		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			return mockSSHConfig, nil
		}

		deployment := &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}
		deployment.Machines["machine1"] = NewMockMachine(
			"machine1",
			testPrivateKeyPath,
		)
		deployment.Machines["machine2"] = NewMockMachine(
			"machine2",
			testPrivateKeyPath,
		).Machine
		deployment.Machines["machine3"] = NewMockMachine(
			"machine3",
			testPrivateKeyPath,
		).Machine

		m := display.GetGlobalModelFunc()
		m.Deployment = deployment

		err := cd.ProvisionAllMachinesWithPackages(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to provision packages on all machines")
		assert.Contains(t, err.Error(), "failed to push file")
	})
	t.Run("error during provisioning", func(t *testing.T) {
		mockProvider := new(MockProvider)
		cd := NewClusterDeployer(mockProvider)

		mockSSHConfig := new(sshutils.MockSSHConfig)
		mockSSHConfig.On("PushFile", ctx, "/tmp/install-docker.sh", mock.Anything, mock.Anything).
			Return(nil)
		mockSSHConfig.On("ExecuteCommand", ctx, "sudo /tmp/install-docker.sh", mock.Anything).
			Return("", errors.New("failed to execute command"))
		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			return mockSSHConfig, nil
		}

		deployment := &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}
		deployment.Machines["machine1"] = NewMockMachine(
			"machine1",
			testPrivateKeyPath,
		)
		deployment.Machines["machine2"] = NewMockMachine(
			"machine2",
			testPrivateKeyPath,
		)
		deployment.Machines["machine3"] = NewMockMachine(
			"machine3",
			testPrivateKeyPath,
		)

		m := display.GetGlobalModelFunc()
		m.Deployment = deployment

		err := cd.ProvisionAllMachinesWithPackages(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to provision packages on all machines")
		assert.Contains(t, err.Error(), "failed to execute command")
	})
}

func TestProvisionBacalhauCluster(t *testing.T) {
	ctx := context.Background()

	t.Run("successful provisioning", func(t *testing.T) {
		_,
			cleanupPublicKey,
			testPrivateKeyPath,
			cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
		defer cleanupPublicKey()
		defer cleanupPrivateKey()

		viper.Set("general.project_prefix", "test-project")

		mockSSHConfig := new(sshutils.MockSSHConfig)
		mockSSHConfig.On("PushFile", mock.Anything, "/tmp/install-docker.sh", mock.Anything, mock.Anything).
			Return(nil).
			Times(3)
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-docker.sh", mock.Anything).
			Return("", nil).
			Times(3)
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker run hello-world").
			Return("", nil).
			Times(3)
		mockSSHConfig.On("PushFile", mock.Anything, "/tmp/install-core-packages.sh", mock.Anything, mock.Anything).
			Return(nil).
			Times(3)
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-core-packages.sh", mock.Anything).
			Return("", nil).
			Times(3)
		mockSSHConfig.On("PushFile", mock.Anything, "/tmp/get-node-config-metadata.sh", mock.Anything, mock.Anything).
			Return(nil).
			Times(3)
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/get-node-config-metadata.sh", mock.Anything).
			Return("", nil).
			Times(3)
		mockSSHConfig.On("PushFile", mock.Anything, "/tmp/install-bacalhau.sh", mock.Anything, mock.Anything).
			Return(nil).
			Times(3)
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-bacalhau.sh", mock.Anything).
			Return("", nil).
			Times(3)
		mockSSHConfig.On("PushFile", mock.Anything, "/tmp/install-run-bacalhau.sh", mock.Anything, mock.Anything).
			Return(nil).
			Times(3)
		mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-run-bacalhau.sh", mock.Anything).
			Return("", nil).
			Times(3)
		mockSSHConfig.On("InstallSystemdService", mock.Anything, "bacalhau", mock.Anything).
			Return(nil).
			Times(3)
		mockSSHConfig.On("RestartService", mock.Anything, "bacalhau").
			Return(nil).
			Times(3)

		stringMatcher := mock.MatchedBy(func(s string) bool {
			return strings.Contains(s, "bacalhau node list --output json --api-host")
		})
		mockSSHConfig.On("ExecuteCommand", mock.Anything, stringMatcher, mock.Anything).
			Return(`[{"name":"orch","address":"1.1.1.1"}]`, nil)

		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			return mockSSHConfig, nil
		}

		m := display.GetGlobalModelFunc()

		orchestratorMachine := NewMockMachine("orch", testPrivateKeyPath)
		orchestratorMachine.Name = "orch"
		orchestratorMachine.Orchestrator = true
		orchestratorMachine.PublicIP = "1.1.1.1"

		workerMachine := NewMockMachine("worker1", testPrivateKeyPath)

		orchestratorMachine.On("SetServiceState", "Bacalhau", models.ServiceStateUpdating).Return()
		orchestratorMachine.On("SetServiceState", "Bacalhau", models.ServiceStateSucceeded).Return()
		orchestratorMachine.On("SetComplete").Return()

		workerMachine.On("SetServiceState", "Bacalhau", models.ServiceStateUpdating).Return()
		workerMachine.On("SetServiceState", "Bacalhau", models.ServiceStateSucceeded).Return()
		workerMachine.On("SetComplete").Return()

		m.Deployment.Machines["orch"] = orchestratorMachine.Machine
		m.Deployment.Machines["worker1"] = workerMachine.Machine

		cd := NewClusterDeployer(models.DeploymentTypeAzure)
		err := cd.ProvisionBacalhauCluster(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "1.1.1.1", m.Deployment.OrchestratorIP)
		orchestratorMachine.AssertExpectations(t)
		workerMachine.AssertExpectations(t)
		mockSSHConfig.AssertExpectations(t)
	})
}

func TestFindOrchestratorMachine(t *testing.T) {
	cd := NewClusterDeployer(nil)

	t.Run("single orchestrator", func(t *testing.T) {
		m := display.GetGlobalModelFunc()
		m.Deployment.Machines["orch"] = &models.Machine{Name: "orch", Orchestrator: true}
		orchestrator, err := cd.FindOrchestratorMachine()

		assert.NoError(t, err)
		assert.NotNil(t, orchestrator)
		assert.Equal(t, "orch", orchestrator.GetName())
	})

	t.Run("no orchestrator", func(t *testing.T) {
		m := display.GetGlobalModelFunc()
		m.Deployment.Machines["worker1"] = &models.Machine{Name: "worker1"}
		m.Deployment.Machines["worker2"] = &models.Machine{Name: "worker2"}
		orchestrator, err := cd.FindOrchestratorMachine()

		assert.Error(t, err)
		assert.Nil(t, orchestrator)
		assert.Contains(t, err.Error(), "no orchestrator node found")
	})

	t.Run("multiple orchestrators", func(t *testing.T) {
		m := display.GetGlobalModelFunc()
		m.Deployment.Machines["orch1"] = &models.Machine{Name: "orch1", Orchestrator: true}
		m.Deployment.Machines["orch2"] = &models.Machine{Name: "orch2", Orchestrator: true}
		m.Deployment.Machines["worker1"] = &models.Machine{Name: "worker1"}

		orchestrator, err := cd.FindOrchestratorMachine()

		assert.Error(t, err)
		assert.Nil(t, orchestrator)
		assert.Contains(t, err.Error(), "multiple orchestrator nodes found")
	})
}
