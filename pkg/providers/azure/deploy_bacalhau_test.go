package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersAzureDeployBacalhauTestSuite struct {
	suite.Suite
	ctx           context.Context
	deployment    *models.Deployment
	deployer      *common.ClusterDeployer
	mockSSHConfig *sshutils.MockSSHConfig
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) SetupTest() {
	s.ctx = context.Background()
	viper.Set("general.project_prefix", "test-project")
	s.deployment = &models.Deployment{
		Machines: make(map[string]models.Machiner),
		Azure: &models.AzureConfig{
			ResourceGroupName: "test-rg-name",
		},
	}
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: s.deployment,
		}
	}
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return s.mockSSHConfig, nil
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) setupTestBacalhauDeployer(
	machines map[string]models.Machiner,
	sshBehavior sshutils.ExpectedSSHBehavior,
) {
	s.mockSSHConfig = sshutils.NewMockSSHConfigWithBehavior(sshBehavior)
	s.deployment.SetMachines(machines)
	s.deployment.OrchestratorIP = "1.2.3.4"
	s.deployer = common.NewClusterDeployer(s.deployment.DeploymentType)
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestFindOrchestratorMachine() {
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

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.setupTestBacalhauDeployer(tt.machines, sshutils.ExpectedSSHBehavior{})
			machine, err := s.deployer.FindOrchestratorMachine()

			if tt.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedError)
			} else {
				s.NoError(err)
				s.NotNil(machine)
				s.True(machine.IsOrchestrator())
			}
		})
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestSetupNodeConfigMetadata() {
	s.SetupTest()
	s.deployment.SetMachines(map[string]models.Machiner{
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

	s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

	err := s.deployer.SetupNodeConfigMetadata(
		s.ctx,
		s.deployment.GetMachine("test"),
		s.mockSSHConfig,
		"compute",
	)

	s.NoError(err)
	s.mockSSHConfig.AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestInstallBacalhau() {
	s.SetupTest()
	s.deployment.SetMachines(map[string]models.Machiner{
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

	s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

	err := s.deployer.InstallBacalhau(s.ctx, s.mockSSHConfig)

	s.NoError(err)
	s.mockSSHConfig.AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestInstallBacalhauRun() {
	s.SetupTest()
	s.deployment.SetMachine("test", &models.Machine{Name: "test", Orchestrator: true})

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

	s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

	err := s.deployer.InstallBacalhauRunScript(s.ctx, s.mockSSHConfig)

	s.NoError(err)
	s.mockSSHConfig.AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestSetupBacalhauService() {
	s.SetupTest()
	s.deployment.SetMachine("test", &models.Machine{Name: "test", Orchestrator: true})

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

	s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

	err := s.deployer.SetupBacalhauService(s.ctx, s.mockSSHConfig)

	s.NoError(err)
	s.mockSSHConfig.AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestVerifyBacalhauDeployment() {
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
		s.Run(tt.name, func() {
			s.SetupTest()
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

			s.deployment.SetMachines(map[string]models.Machiner{
				"test": &models.Machine{Name: "test"},
			})

			s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

			err := s.deployer.VerifyBacalhauDeployment(s.ctx, s.mockSSHConfig, "0.0.0.0")

			if tt.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedError)
			} else {
				s.NoError(err)
			}
			s.mockSSHConfig.AssertExpectations(s.T())
		})
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployBacalhauNode() {
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
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: "[]",
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
					Times: 2,
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
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: "[]",
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
					Times: 2,
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
		s.Run(tt.name, func() {
			s.SetupTest()
			s.setupTestBacalhauDeployer(tt.machines, tt.sshBehavior)

			var err error
			if tt.nodeType == "requester" {
				err = s.deployer.ProvisionOrchestrator(s.ctx, tt.machines["test"].GetName())
			} else {
				err = s.deployer.ProvisionWorker(s.ctx, tt.machines["test"].GetName())
			}

			if tt.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedError)
			} else {
				s.NoError(err)
				s.Equal(models.ServiceStateSucceeded, s.deployment.Machines["test"].GetServiceState(models.ServiceTypeBacalhau.Name))
			}
		})
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployOrchestrator() {
	s.SetupTest()
	hostname := "orch"
	ip := "1.2.3.4"
	location := "eastus"
	orchestrators := "0.0.0.0"
	vmSize := "Standard_DS4_v2"

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
			`    /usr/local/bin/bacalhau serve \`,
		},
	}

	s.deployment.SetMachines(map[string]models.Machiner{
		"orch": &models.Machine{
			Name:         hostname,
			Orchestrator: true,
			PublicIP:     ip,
			VMSize:       vmSize,
			Location:     location,
		},
	})

	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{}, // Not setting behaviors here because we want to get the script being pushed
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              mock.Anything,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd: fmt.Sprintf(
					"bacalhau node list --output json --api-host %s",
					orchestrators,
				),
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            1,
			},
			{
				Cmd:    "sudo bacalhau config list --output json",
				Output: "[]",
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
			Times: 2,
		},
	}

	s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

	s.mockSSHConfig.On("WaitForSSH", s.ctx, mock.Anything, mock.Anything).Return(nil).Once()

	renderedScripts := make(map[string][]byte)
	s.mockSSHConfig.On("PushFile", s.ctx, "/tmp/get-node-config-metadata.sh", mock.Anything, true, mock.Anything).
		Run(func(args mock.Arguments) {
			renderedScripts["get-node-config-metadata.sh"] = args.Get(2).([]byte)
		}).
		Return(nil).
		Once()
	s.mockSSHConfig.On("PushFile", s.ctx, "/tmp/install-bacalhau.sh", mock.Anything, true, mock.Anything).
		Run(func(args mock.Arguments) {
			renderedScripts["install-bacalhau.sh"] = args.Get(2).([]byte)
		}).
		Return(nil).
		Once()
	s.mockSSHConfig.On("PushFile", s.ctx, "/tmp/install-run-bacalhau.sh", mock.Anything, true, mock.Anything).
		Run(func(args mock.Arguments) {
			renderedScripts["install-run-bacalhau.sh"] = args.Get(2).([]byte)
		}).
		Return(nil).
		Once()

	err := s.deployer.ProvisionOrchestrator(s.ctx, "orch")

	s.NoError(err)
	s.Equal(
		models.ServiceStateSucceeded,
		s.deployment.Machines["orch"].GetServiceState(models.ServiceTypeBacalhau.Name),
	)
	s.NotEmpty(s.deployment.OrchestratorIP)

	filesToTest := map[string]string{
		"get-node-config-metadata.sh": fmt.Sprintf(fileToTestMetadata, location, ip, orchestrators),
		"install-bacalhau.sh":         fileToTestInstall,
		"install-run-bacalhau.sh":     fileToTestServe,
	}
	for fileToTest := range filesToTest {
		for _, expectedLine := range expectedLines[fileToTest] {
			s.Contains(string(renderedScripts[fileToTest]), expectedLine)
		}
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployWorkers() {
	s.SetupTest()
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

	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/get-node-config-metadata.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-bacalhau.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-run-bacalhau.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
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
			{
				Cmd:    "sudo bacalhau config list --output json",
				Output: "[]",
				Error:  nil,
				Times:  3,
			},
			{
				Cmd:              mock.Anything,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            9,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 3,
		},
		RestartServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 6,
		},
	}
	s.deployment.SetMachines(machines)
	s.setupTestBacalhauDeployer(s.deployment.GetMachines(), sshBehavior)

	var err error
	for _, machine := range machines {
		if machine.IsOrchestrator() {
			err = s.deployer.ProvisionOrchestrator(s.ctx, machine.GetName())
		} else {
			err = s.deployer.ProvisionWorker(s.ctx, machine.GetName())
		}
		s.NoError(err)
		s.Equal(
			models.ServiceStateSucceeded,
			s.deployment.Machines[machine.GetName()].GetServiceState(
				models.ServiceTypeBacalhau.Name,
			),
		)
	}

	s.mockSSHConfig.AssertExpectations(s.T())
}

func TestDeployBacalhauSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAzureDeployBacalhauTestSuite))
}

const fileToTestMetadata = `
cat << EOF > /etc/node-config
MACHINE_TYPE="Standard_DS4_v2"
HOSTNAME="orch"
VCPU_COUNT="$VCPU_COUNT"
MEMORY_GB="$MEMORY_GB"
DISK_GB="$DISK_SIZE"
LOCATION="%s"
IP="%s"
ORCHESTRATORS="%s"
TOKEN=""
NODE_TYPE="requester"
`

const fileToTestInstall = `sudo curl -sSL https://get.bacalhau.org/install.sh?dl="${BACALHAU_INSTALL_ID}" | sudo bash`

const fileToTestServe = `/usr/local/bin/bacalhau serve \`
