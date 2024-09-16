package common

import (
	"context"
	"errors"
	"fmt"
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

// ExpectedMachineBehavior holds the expected outcomes for mock methods
type ExpectedMachineBehavior struct {
	InstallDockerAndCorePackagesError error
	// Add other methods as needed
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
	m.On("SetServiceState", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		m.Machine.SetServiceState(args[0].(string), args[1].(models.ServiceState))
	}).Return()
	m.On("SetComplete").Return()
	return m
}

func NewMockMachineWithBehavior(
	name string,
	privateKeyPath string,
	behavior ExpectedMachineBehavior,
) *MockMachine {
	m := NewMockMachine(name, privateKeyPath)

	m.On("InstallDockerAndCorePackages", mock.Anything).
		Return(behavior.InstallDockerAndCorePackagesError)
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
	_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	viper.Set("general.project_prefix", "test-project")

	t.Run("successful provisioning", func(t *testing.T) {
		cd := NewClusterDeployer(models.DeploymentTypeUnknown)

		sshBehavior := sshutils.ExpectedSSHBehavior{
			PushFileExpectations: []sshutils.PushFileExpectation{
				{
					Dst:              "/tmp/install-docker.sh",
					Executable:       true,
					ProgressCallback: mock.Anything,
					Error:            nil,
					Times:            1,
				},
				{
					Dst:              "/tmp/install-core-packages.sh",
					Executable:       true,
					ProgressCallback: mock.Anything,
					Error:            nil,
					Times:            1,
				},
			},
			ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
				{
					Cmd:              "sudo /tmp/install-docker.sh",
					ProgressCallback: mock.Anything,
					Output:           "",
					Error:            nil,
					Times:            1,
				},
				{
					Cmd:              "sudo docker run hello-world",
					ProgressCallback: mock.Anything,
					Output:           "",
					Error:            nil,
					Times:            1,
				},
				{
					Cmd:              "sudo /tmp/install-core-packages.sh",
					ProgressCallback: mock.Anything,
					Output:           "",
					Error:            nil,
					Times:            1,
				},
			},
		}

		// Mock SSHConfig to return the expected behavior
		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			return sshutils.NewMockSSHConfigWithBehavior(sshBehavior), nil
		}

		deployment := &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}

		behavior := ExpectedMachineBehavior{
			InstallDockerAndCorePackagesError: nil,
		}

		machine1 := NewMockMachineWithBehavior("machine1", testPrivateKeyPath, behavior)
		machine2 := NewMockMachineWithBehavior("machine2", testPrivateKeyPath, behavior)
		machine3 := NewMockMachineWithBehavior("machine3", testPrivateKeyPath, behavior)

		deployment.Machines["machine1"] = machine1
		deployment.Machines["machine2"] = machine2
		deployment.Machines["machine3"] = machine3

		m := display.GetGlobalModelFunc()
		m.Deployment = deployment

		err := cd.ProvisionAllMachinesWithPackages(ctx)

		assert.NoError(t, err)
	})

	t.Run("error with SSH", func(t *testing.T) {
		cd := NewClusterDeployer(models.DeploymentTypeUnknown)

		sshBehavior := sshutils.ExpectedSSHBehavior{
			PushFileExpectations: []sshutils.PushFileExpectation{
				{
					Dst:              "/tmp/install-docker.sh",
					Executable:       true,
					ProgressCallback: mock.Anything,
					Error:            errors.New("failed to push file"),
					Times:            1,
				},
			},
		}

		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			return sshutils.NewMockSSHConfigWithBehavior(sshBehavior), nil
		}

		deployment := &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}

		behavior := ExpectedMachineBehavior{
			InstallDockerAndCorePackagesError: fmt.Errorf("failed to push file"),
		}

		machine1 := NewMockMachineWithBehavior("machine1", testPrivateKeyPath, behavior)
		machine2 := NewMockMachineWithBehavior("machine2", testPrivateKeyPath, behavior)
		machine3 := NewMockMachineWithBehavior("machine3", testPrivateKeyPath, behavior)

		deployment.Machines["machine1"] = machine1
		deployment.Machines["machine2"] = machine2
		deployment.Machines["machine3"] = machine3

		m := display.GetGlobalModelFunc()
		m.Deployment = deployment

		err := cd.ProvisionAllMachinesWithPackages(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to provision packages on all machines")
		assert.Contains(t, err.Error(), "failed to push file")
	})

	t.Run("error during provisioning", func(t *testing.T) {
		cd := NewClusterDeployer(models.DeploymentTypeUnknown)

		sshBehaviorSuccess := sshutils.ExpectedSSHBehavior{
			PushFileExpectations: []sshutils.PushFileExpectation{
				{
					Dst:              "/tmp/install-docker.sh",
					Executable:       true,
					ProgressCallback: mock.Anything,
					Error:            nil,
					Times:            1,
				},
			},
			ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
				{
					Cmd:              "sudo /tmp/install-docker.sh",
					ProgressCallback: mock.Anything,
					Output:           "",
					Error:            nil,
					Times:            1,
				},
				{
					Cmd:              "sudo docker run hello-world",
					ProgressCallback: mock.Anything,
					Output:           "",
					Error:            nil,
					Times:            1,
				},
			},
		}

		sshBehaviorFail := sshutils.ExpectedSSHBehavior{
			PushFileExpectations: []sshutils.PushFileExpectation{
				{
					Dst:              "/tmp/install-docker.sh",
					Executable:       true,
					ProgressCallback: mock.Anything,
					Error:            nil,
					Times:            1,
				},
			},
			ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
				{
					Cmd:              "sudo /tmp/install-docker.sh",
					ProgressCallback: mock.Anything,
					Output:           "",
					Error:            errors.New("failed to execute command"),
					Times:            1,
				},
			},
		}

		// Map to assign different behaviors to different machines
		sshBehaviorMap := map[string]sshutils.ExpectedSSHBehavior{
			"machine1": sshBehaviorFail,
			"machine2": sshBehaviorSuccess,
			"machine3": sshBehaviorSuccess,
		}

		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			// Use the hostname to select the appropriate behavior
			behavior, exists := sshBehaviorMap[host]
			if !exists {
				behavior = sshBehaviorSuccess
			}
			return sshutils.NewMockSSHConfigWithBehavior(behavior), nil
		}

		deployment := &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}

		behaviorSuccess := ExpectedMachineBehavior{
			InstallDockerAndCorePackagesError: nil,
		}
		behaviorFail := ExpectedMachineBehavior{
			InstallDockerAndCorePackagesError: errors.New("failed to execute command"),
		}

		machine1 := NewMockMachineWithBehavior("machine1", testPrivateKeyPath, behaviorFail)
		machine2 := NewMockMachineWithBehavior("machine2", testPrivateKeyPath, behaviorSuccess)
		machine3 := NewMockMachineWithBehavior("machine3", testPrivateKeyPath, behaviorSuccess)

		deployment.Machines["machine1"] = machine1
		deployment.Machines["machine2"] = machine2
		deployment.Machines["machine3"] = machine3

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
		_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
		defer cleanupPublicKey()
		defer cleanupPrivateKey()

		viper.Set("general.project_prefix", "test-project")

		// Define SSH behavior per machine
		sshBehaviorMap := map[string]sshutils.ExpectedSSHBehavior{
			"1.1.1.1": { // Orchestrator IP
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:              mock.Anything,
						Executable:       true,
						ProgressCallback: mock.Anything,
						Error:            nil,
						Times:            3,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:              mock.Anything,
						ProgressCallback: mock.Anything,
						Output:           "",
						Error:            nil,
						Times:            3,
					},
					{
						CmdMatcher: func(cmd string) bool {
							return strings.Contains(
								cmd,
								"bacalhau node list --output json --api-host",
							)
						},
						ProgressCallback: mock.Anything,
						Output:           `[{"name":"orch","address":"1.1.1.1"}]`,
						Error:            nil,
						Times:            1,
					},
				},
				InstallSystemdServiceExpectation: &sshutils.Expectation{Error: nil, Times: 1},
				RestartServiceExpectation:        &sshutils.Expectation{Error: nil, Times: 1},
			},
			"2.2.2.2": { // Worker IP
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:              mock.Anything,
						Executable:       true,
						ProgressCallback: mock.Anything,
						Error:            nil,
						Times:            3,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:              mock.Anything,
						ProgressCallback: mock.Anything,
						Output:           "",
						Error:            nil,
						Times:            3,
					},
					{
						CmdMatcher: func(cmd string) bool {
							return strings.Contains(
								cmd,
								"bacalhau node list --output json --api-host",
							)
						},
						ProgressCallback: mock.Anything,
						Output:           `[{"name":"worker1","address":"2.2.2.2"}]`,
						Error:            nil,
						Times:            1,
					},
				},
				InstallSystemdServiceExpectation: &sshutils.Expectation{Error: nil, Times: 1},
				RestartServiceExpectation:        &sshutils.Expectation{Error: nil, Times: 1},
			},
		}

		// Mock SSHConfig to return the expected behavior per machine
		sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
			behavior, exists := sshBehaviorMap[host]
			if !exists {
				return nil, fmt.Errorf("no SSH behavior defined for host %s", host)
			}
			return sshutils.NewMockSSHConfigWithBehavior(behavior), nil
		}

		m := display.GetGlobalModelFunc()

		behavior := ExpectedMachineBehavior{
			InstallDockerAndCorePackagesError: nil,
		}

		orchestratorMachine := NewMockMachineWithBehavior("orch", testPrivateKeyPath, behavior)
		orchestratorMachine.Name = "orch"
		orchestratorMachine.Orchestrator = true
		orchestratorMachine.PublicIP = "1.1.1.1"

		workerMachine := NewMockMachineWithBehavior("worker1", testPrivateKeyPath, behavior)
		workerMachine.Name = "worker1"
		workerMachine.PublicIP = "2.2.2.2"

		// Set expectations on the machines
		orchestratorMachine.On("SetServiceState", "Bacalhau", models.ServiceStateUpdating).Return()
		orchestratorMachine.On("SetServiceState", "Bacalhau", models.ServiceStateSucceeded).Return()
		orchestratorMachine.On("SetComplete").Return()

		workerMachine.On("SetServiceState", "Bacalhau", models.ServiceStateUpdating).Return()
		workerMachine.On("SetServiceState", "Bacalhau", models.ServiceStateSucceeded).Return()
		workerMachine.On("SetComplete").Return()

		m.Deployment.Machines["orch"] = orchestratorMachine
		m.Deployment.Machines["worker1"] = workerMachine

		cd := NewClusterDeployer(models.DeploymentTypeUnknown)
		err := cd.ProvisionBacalhauCluster(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "1.1.1.1", m.Deployment.OrchestratorIP)
		orchestratorMachine.AssertExpectations(t)
		workerMachine.AssertExpectations(t)
		// We cannot assert expectations on the SSHConfig directly since it's created per machine
	})
}

func TestFindOrchestratorMachine(t *testing.T) {
	cd := NewClusterDeployer(models.DeploymentTypeUnknown)

	viper.Set("general.project_prefix", "test-project")

	t.Run("single orchestrator", func(t *testing.T) {
		m := display.GetGlobalModelFunc()
		// Reinitialize Deployment to ensure a clean state
		m.Deployment = &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}
		m.Deployment.Machines["orch"] = &models.Machine{Name: "orch", Orchestrator: true}
		orchestrator, err := cd.FindOrchestratorMachine()

		assert.NoError(t, err)
		assert.NotNil(t, orchestrator)
		assert.Equal(t, "orch", orchestrator.GetName())
	})

	t.Run("no orchestrator", func(t *testing.T) {
		m := display.GetGlobalModelFunc()
		// Reinitialize Deployment to ensure a clean state
		m.Deployment = &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}
		m.Deployment.Machines["worker1"] = &models.Machine{Name: "worker1"}
		m.Deployment.Machines["worker2"] = &models.Machine{Name: "worker2"}
		orchestrator, err := cd.FindOrchestratorMachine()

		assert.Error(t, err)
		assert.Nil(t, orchestrator)
		assert.Contains(t, err.Error(), "no orchestrator node found")
	})

	t.Run("multiple orchestrators", func(t *testing.T) {
		m := display.GetGlobalModelFunc()
		// Reinitialize Deployment to ensure a clean state
		m.Deployment = &models.Deployment{
			Machines: make(map[string]models.Machiner),
		}
		m.Deployment.Machines["orch1"] = &models.Machine{Name: "orch1", Orchestrator: true}
		m.Deployment.Machines["orch2"] = &models.Machine{Name: "orch2", Orchestrator: true}
		m.Deployment.Machines["worker1"] = &models.Machine{Name: "worker1"}

		orchestrator, err := cd.FindOrchestratorMachine()

		assert.Error(t, err)
		assert.Nil(t, orchestrator)
		assert.Contains(t, err.Error(), "multiple orchestrator nodes found")
	})
}
