package azure

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestBacalhauDeployer(
	machines map[string]models.Machiner,
	sshBehavior sshutils.ExpectedSSHBehavior,
) (*common.ClusterDeployer, *sshutils.MockSSHConfig) {
	mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(sshBehavior)

	m := display.GetGlobalModelFunc()
	m.Deployment.Machines = machines
	m.Deployment.OrchestratorIP = "1.2.3.4"

	deployer := common.NewClusterDeployer(models.DeploymentTypeAzure)

	// Override the SSH config creation function
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string,
	) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	return deployer, mockSSHConfig
}

func TestFindOrchestratorMachine(t *testing.T) {
	tests := []struct {
		name          string
		machines      map[string]models.Machiner
		expectedError string
	}{
		{
			name: "Single orchestrator",
			machines: map[string]models.Machiner{
				"orch":   &models.Machine{Name: "orch", Orchestrator: true},
				"worker": &models.Machine{Name: "worker"},
			},
			expectedError: "",
		},
		{
			name: "No orchestrator",
			machines: map[string]models.Machiner{
				"worker1": &models.Machine{Name: "worker1"},
				"worker2": &models.Machine{Name: "worker2"},
			},
			expectedError: "no orchestrator node found",
		},
		{
			name: "Multiple orchestrators",
			machines: map[string]models.Machiner{
				"orch1": &models.Machine{Name: "orch1", Orchestrator: true},
				"orch2": &models.Machine{Name: "orch2", Orchestrator: true},
			},
			expectedError: "multiple orchestrator nodes found",
		},
	}

	viper.Set("general.project_prefix", "test-project")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := display.GetGlobalModelFunc()
			m.Deployment.SetMachines(map[string]models.Machiner{})
			m.Deployment.SetMachines(tt.machines)
			deployer, _ := setupTestBacalhauDeployer(
				m.Deployment.GetMachines(),
				sshutils.ExpectedSSHBehavior{},
			)
			machine, err := deployer.FindOrchestratorMachine()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, machine)
				assert.True(t, machine.IsOrchestrator())
			}
		})
	}
}

func TestSetupNodeConfigMetadata(t *testing.T) {
	viper.Set("general.project_prefix", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()

	m.Deployment.SetMachines(map[string]models.Machiner{
		"test": &models.Machine{Name: "test", VMSize: "Standard_DS4_v2", Location: "eastus2"},
	})

	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/get-node-config-metadata.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            1,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              "sudo /tmp/get-node-config-metadata.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            1,
			},
		},
	}

	deployer, mockSSHConfig := setupTestBacalhauDeployer(m.Deployment.GetMachines(), sshBehavior)

	err := deployer.SetupNodeConfigMetadata(
		ctx,
		m.Deployment.GetMachine("test"),
		mockSSHConfig,
		"compute",
	)

	assert.NoError(t, err)
	mockSSHConfig.AssertExpectations(t)
}

func TestInstallBacalhau(t *testing.T) {
	viper.Set("general.project_prefix", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.SetMachines(map[string]models.Machiner{
		"test": &models.Machine{Name: "test", Orchestrator: true},
	})

	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/install-bacalhau.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            1,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              "sudo /tmp/install-bacalhau.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            1,
			},
		},
	}

	deployer, mockSSHConfig := setupTestBacalhauDeployer(m.Deployment.GetMachines(), sshBehavior)

	err := deployer.InstallBacalhau(
		ctx,
		mockSSHConfig,
	)

	assert.NoError(t, err)
	mockSSHConfig.AssertExpectations(t)
}

func TestInstallBacalhauRun(t *testing.T) {
	viper.Set("general.project_prefix", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.SetMachine("test", &models.Machine{Name: "test", Orchestrator: true})

	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/install-run-bacalhau.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            1,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              "sudo /tmp/install-run-bacalhau.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            1,
			},
		},
	}

	deployer, mockSSHConfig := setupTestBacalhauDeployer(m.Deployment.GetMachines(), sshBehavior)

	err := deployer.InstallBacalhauRunScript(
		ctx,
		mockSSHConfig,
	)

	assert.NoError(t, err)
	mockSSHConfig.AssertExpectations(t)
}

func TestSetupBacalhauService(t *testing.T) {
	viper.Set("general.project_prefix", "test-project")
	m := display.GetGlobalModelFunc()
	ctx := context.Background()
	m.Deployment.SetMachine("test", &models.Machine{Name: "test", Orchestrator: true})

	sshBehavior := sshutils.ExpectedSSHBehavior{
		InstallSystemdServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 1,
		},
		RestartServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 1,
		},
	}

	deployer, mockSSHConfig := setupTestBacalhauDeployer(m.Deployment.GetMachines(), sshBehavior)

	err := deployer.SetupBacalhauService(
		ctx,
		mockSSHConfig,
	)

	assert.NoError(t, err)
	mockSSHConfig.AssertExpectations(t)
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
			sshBehavior := sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:              "bacalhau node list --output json --api-host 0.0.0.0",
						ProgressCallback: mock.Anything,
						Output:           tt.nodeListOutput,
						Error:            nil,
						Times:            1,
					},
				},
			}

			m := display.GetGlobalModelFunc()
			m.Deployment.SetMachines(map[string]models.Machiner{
				"test": &models.Machine{Name: "test"},
			})

			deployer, mockSSHConfig := setupTestBacalhauDeployer(
				m.Deployment.GetMachines(),
				sshBehavior,
			)

			err := deployer.VerifyBacalhauDeployment(
				ctx,
				mockSSHConfig,
				"0.0.0.0",
			)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			mockSSHConfig.AssertExpectations(t)
		})
	}
}

func TestDeployBacalhauNode(t *testing.T) {
	viper.Set("general.project_prefix", "test-project")

	ctx := context.Background()
	tests := []struct {
		name          string
		nodeType      string
		sshBehavior   sshutils.ExpectedSSHBehavior
		machines      map[string]models.Machiner
		expectedError string
	}{
		{
			name:     "Successful orchestrator deployment",
			nodeType: "requester",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{Dst: mock.Anything, Executable: true, Error: nil, Times: 3},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{Cmd: mock.Anything, Output: "", Error: nil, Times: 3},
					{
						Cmd:    "bacalhau node list --output json --api-host 0.0.0.0",
						Output: `[{"id": "node1"}]`,
						Error:  nil,
						Times:  1,
					},
				},
				InstallSystemdServiceExpectation: &sshutils.Expectation{
					Error: nil,
					Times: 1,
				},
				RestartServiceExpectation: &sshutils.Expectation{
					Error: nil,
					Times: 1,
				},
			},
			machines: map[string]models.Machiner{
				"test": &models.Machine{Name: "test", Orchestrator: true, PublicIP: "1.2.3.4"},
			},
			expectedError: "",
		},
		{
			name:     "Failed orchestrator deployment",
			nodeType: "requester",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        mock.Anything,
						Executable: true,
						Error:      fmt.Errorf("push file error"),
						Times:      1,
					},
				},
			},
			machines: map[string]models.Machiner{
				"test": &models.Machine{Name: "test", Orchestrator: true, PublicIP: "1.2.3.4"},
			},
			expectedError: "failed to push node config metadata script",
		},
		{
			name:     "Successful worker deployment",
			nodeType: "compute",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        mock.Anything,
						Executable: true,
						Error:      nil,
						Times:      3,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{Cmd: mock.Anything, Output: "", Error: nil, Times: 3},
					{
						Cmd:    "bacalhau node list --output json --api-host 1.2.3.4",
						Output: `[{"id": "node1"}]`,
						Error:  nil,
						Times:  1,
					},
				},
				InstallSystemdServiceExpectation: &sshutils.Expectation{
					Error: nil,
					Times: 1,
				},
				RestartServiceExpectation: &sshutils.Expectation{
					Error: nil,
					Times: 1,
				},
			},
			machines: map[string]models.Machiner{
				"test": &models.Machine{
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
			m.Deployment.SetMachines(tt.machines)
			deployer, _ := setupTestBacalhauDeployer(m.Deployment.GetMachines(), tt.sshBehavior)

			var err error
			if tt.nodeType == "requester" {
				err = deployer.ProvisionOrchestrator(
					ctx,
					tt.machines["test"].GetName(),
				)
			} else {
				err = deployer.ProvisionWorker(
					ctx,
					tt.machines["test"].GetName(),
				)
			}

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, models.ServiceStateSucceeded, m.Deployment.Machines["test"].GetServiceState("Bacalhau"))
			}
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

	viper.Set("general.project_prefix", "test-project")

	expectedLines := map[string][]string{
		"get-node-config-metadata.sh": {
			`cat << EOF > /etc/node-config`,
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

	m := display.GetGlobalModelFunc()
	m.Deployment.SetMachines(map[string]models.Machiner{
		"orch": &models.Machine{
			Name:         hostname,
			Orchestrator: true,
			PublicIP:     ip,
			VMSize:       vmSize,
			Location:     location,
		},
	})

	// Define ExpectedSSHBehavior with updated method signatures
	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/get-node-config-metadata.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            1,
			},
			{
				Dst:              "/tmp/install-bacalhau.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            1,
			},
			{
				Dst:              "/tmp/install-run-bacalhau.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            1,
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
				Cmd:              "bacalhau node list --output json --api-host 0.0.0.0",
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            1,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 1,
		},
		RestartServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 1,
		},
	}

	// Create a map to capture the rendered scripts
	renderedScripts := make(map[string][]byte)

	// Create mockSSHConfig with behavior
	mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(sshBehavior)

	// Set up WaitForSSH expectations
	mockSSHConfig.On("WaitForSSH", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	// Override sshutils.NewSSHConfigFunc to return our mockSSHConfig
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	// Set up PushFile to capture the rendered scripts
	mockSSHConfig.On("PushFile", ctx, "/tmp/get-node-config-metadata.sh", mock.Anything, true, mock.Anything).
		Run(func(args mock.Arguments) {
			renderedScripts["get-node-config-metadata.sh"] = args.Get(2).([]byte)
		}).
		Return(nil).
		Once()
	mockSSHConfig.On("PushFile", ctx, "/tmp/install-bacalhau.sh", mock.Anything, true, mock.Anything).
		Run(func(args mock.Arguments) {
			renderedScripts["install-bacalhau.sh"] = args.Get(2).([]byte)
		}).
		Return(nil).
		Once()
	mockSSHConfig.On("PushFile", ctx, "/tmp/install-run-bacalhau.sh", mock.Anything, true, mock.Anything).
		Run(func(args mock.Arguments) {
			renderedScripts["install-run-bacalhau.sh"] = args.Get(2).([]byte)
		}).
		Return(nil).
		Once()

	// Create a ClusterDeployer
	deployer := common.NewClusterDeployer(models.DeploymentTypeAzure)

	// Provision the orchestrator
	err := deployer.ProvisionOrchestrator(ctx, "orch")

	assert.NoError(t, err)
	assert.Equal(
		t,
		models.ServiceStateSucceeded,
		m.Deployment.Machines["orch"].GetServiceState("Bacalhau"),
	)
	assert.NotEmpty(t, m.Deployment.OrchestratorIP)

	// Check the content of each script
	filesToTest := []string{
		"get-node-config-metadata.sh",
		"install-bacalhau.sh",
		"install-run-bacalhau.sh",
	}
	for _, fileToTest := range filesToTest {
		for _, expectedLine := range expectedLines[fileToTest] {
			if !bytes.Contains(renderedScripts[fileToTest], []byte(expectedLine+"\n")) {
				assert.Fail(t, fmt.Sprintf("Expected line not found: %s", expectedLine), fileToTest)
			}
		}
	}
}

func TestDeployWorkers(t *testing.T) {
	ctx := context.Background()

	viper.Set("general.project_prefix", "test-project")

	machines := map[string]models.Machiner{
		"orch": &models.Machine{Name: "orch", Orchestrator: true, PublicIP: "1.2.3.4"},
		"worker1": &models.Machine{
			Name:           "worker1",
			Orchestrator:   false,
			PublicIP:       "2.3.4.5",
			OrchestratorIP: "1.2.3.4",
		},
		"worker2": &models.Machine{
			Name:           "worker2",
			Orchestrator:   false,
			PublicIP:       "3.4.5.6",
			OrchestratorIP: "1.2.3.4",
		},
	}

	// Define ExpectedSSHBehavior with updated method signatures
	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              mock.Anything,
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            9,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              mock.Anything,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            9,
			},
			{
				Cmd:              "bacalhau node list --output json --api-host 1.2.3.4",
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            2,
			},
			{
				Cmd:              "bacalhau node list --output json --api-host 0.0.0.0",
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            1,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 3,
		},
		RestartServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 3,
		},
	}

	m := display.GetGlobalModelFunc()
	m.Deployment.SetMachines(machines)

	// Create mockSSHConfig with behavior
	mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(sshBehavior)

	// Set up WaitForSSH expectations
	mockSSHConfig.On("WaitForSSH", ctx, mock.Anything, mock.Anything).Return(nil).Times(3)

	// Override sshutils.NewSSHConfigFunc to return our mockSSHConfig
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	// Create a ClusterDeployer
	deployer := common.NewClusterDeployer(models.DeploymentTypeAzure)

	var err error
	for _, machine := range machines {
		if machine.IsOrchestrator() {
			err = deployer.ProvisionOrchestrator(ctx, machine.GetName())
		} else {
			err = deployer.ProvisionWorker(ctx, machine.GetName())
		}
		assert.NoError(t, err)
		assert.Equal(
			t,
			models.ServiceStateSucceeded,
			m.Deployment.Machines[machine.GetName()].GetServiceState("Bacalhau"),
		)
	}

	mockSSHConfig.AssertExpectations(t)
}
