package azure

import (
	"context"
	"testing"

	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	sshutils_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersAzureDeployBacalhauTestSuite struct {
	suite.Suite
	ctx        context.Context
	deployment *models.Deployment
	deployer   *common.ClusterDeployer
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) SetupTest() {
	s.ctx = context.Background()
	if err := viper.BindEnv("general.project_prefix"); err != nil {
		s.T().Fatalf("Failed to bind environment variable: %v", err)
	}
	viper.Set("general.project_prefix", "test-project")

	// Create a fresh deployment for each test
	var err error
	s.deployment, err = models.NewDeployment()
	if err != nil {
		s.T().Fatalf("Failed to create deployment: %v", err)
	}
	s.deployer = common.NewClusterDeployer(models.DeploymentTypeAzure)

	// Reset the global model function for each test
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: s.deployment,
		}
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployOrchestrator() {
	// Setup test data
	s.deployment.SetMachines(map[string]models.Machiner{
		"orch": &models.Machine{
			Name:         "orch",
			Orchestrator: true,
			PublicIP:     "1.2.3.4",
			NodeType:     models.BacalhauNodeTypeOrchestrator,
		},
	})

	// Create mock SSH client
	mockSSHClient := new(ssh_mock.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&ssh_mock.MockSSHSessioner{}, nil).Maybe()
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Define expected behavior
	behavior := sshutils.ExpectedSSHBehavior{
		ConnectExpectation: &sshutils.ConnectExpectation{
			Client: mockSSHClient,
			Error:  nil,
			Times:  2,
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:    "sudo docker run hello-world",
				Times:  1,
				Output: models.ExpectedDockerOutput,
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-docker.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-core-packages.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/get-node-config-metadata.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-bacalhau.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-run-bacalhau.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 0.0.0.0",
				Times:  1,
				Output: `[{"id": "node1"}]`,
				Error:  nil,
			},
		},
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:        "/tmp/install-docker.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/install-core-packages.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/get-node-config-metadata.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/install-bacalhau.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/install-run-bacalhau.sh",
				Executable: true,
				Times:      1,
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
		sshPrivateKeyPath string) (sshutils_interface.SSHConfiger, error) {
		return mockSSH, nil
	}
	defer func() { sshutils.NewSSHConfigFunc = origNewSSHConfigFunc }()

	// Run the test
	err := s.deployer.ProvisionOrchestrator(s.ctx, "orch")
	s.NoError(err)

	// Verify expectations
	mockSSH.(*ssh_mock.MockSSHConfiger).AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployWorker() {
	// Setup test data
	s.deployment.SetMachines(map[string]models.Machiner{
		"worker": &models.Machine{
			Name:           "worker",
			Orchestrator:   false,
			PublicIP:       "2.3.4.5",
			OrchestratorIP: "1.2.3.4",
			NodeType:       models.BacalhauNodeTypeCompute,
		},
	})

	// Create mock SSH client
	mockSSHClient := new(ssh_mock.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&ssh_mock.MockSSHSessioner{}, nil).Maybe()
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Define expected behavior
	behavior := sshutils.ExpectedSSHBehavior{
		ConnectExpectation: &sshutils.ConnectExpectation{
			Client: mockSSHClient,
			Error:  nil,
			Times:  2,
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:    "sudo docker run hello-world",
				Times:  1,
				Output: models.ExpectedDockerOutput,
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-docker.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-core-packages.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/get-node-config-metadata.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-bacalhau.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "sudo /tmp/install-run-bacalhau.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 1.2.3.4",
				Times:  1,
				Output: `[{"id": "node1"}]`,
				Error:  nil,
			},
		},
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:        "/tmp/install-docker.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/install-core-packages.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/get-node-config-metadata.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/install-bacalhau.sh",
				Executable: true,
				Times:      1,
			},
			{
				Dst:        "/tmp/install-run-bacalhau.sh",
				Executable: true,
				Times:      1,
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
		sshPrivateKeyPath string) (sshutils_interface.SSHConfiger, error) {
		return mockSSH, nil
	}
	defer func() { sshutils.NewSSHConfigFunc = origNewSSHConfigFunc }()

	// Run the test
	err := s.deployer.ProvisionWorker(s.ctx, "worker")
	s.NoError(err)

	// Verify expectations
	mockSSH.(*ssh_mock.MockSSHConfiger).AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestSetupNodeConfigMetadata() {
	s.deployment.SetMachines(map[string]models.Machiner{
		"test": &models.Machine{
			Name:     "test",
			VMSize:   "Standard_DS4_v2",
			Location: "eastus2",
			NodeType: models.BacalhauNodeTypeOrchestrator,
		},
	})

	// Create mock SSH client
	mockSSHClient := new(ssh_mock.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&ssh_mock.MockSSHSessioner{}, nil).Maybe()
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Define expected behavior
	behavior := sshutils.ExpectedSSHBehavior{
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:    "sudo /tmp/get-node-config-metadata.sh",
				Times:  1,
				Output: "",
				Error:  nil,
			},
		},
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:        "/tmp/get-node-config-metadata.sh",
				Executable: true,
				Times:      1,
			},
		},
	}

	// Create mock SSH config with behavior
	mockSSH := sshutils.NewMockSSHConfigWithBehavior(behavior)

	err := s.deployer.SetupNodeConfigMetadata(
		s.ctx,
		s.deployment.GetMachine("test"),
		mockSSH,
	)
	s.NoError(err)

	// Verify expectations
	mockSSH.(*ssh_mock.MockSSHConfiger).AssertExpectations(s.T())
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestVerifyBacalhauDeployment() {
	tests := []struct {
		name           string
		nodeListOutput string
		expectError    bool
	}{
		{
			name:           "Successful verification",
			nodeListOutput: `[{"id": "node1"}]`,
			expectError:    false,
		},
		{
			name:           "Empty node list",
			nodeListOutput: `[]`,
			expectError:    true,
		},
		{
			name:           "Invalid JSON",
			nodeListOutput: `invalid json`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Create mock SSH client
			mockSSHClient := new(ssh_mock.MockSSHClienter)
			mockSSHClient.On("Close").Return(nil).Maybe()
			mockSSHClient.On("NewSession").Return(&ssh_mock.MockSSHSessioner{}, nil).Maybe()
			mockSSHClient.On("GetClient").Return(nil).Maybe()

			// Define expected behavior
			behavior := sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "bacalhau node list --output json --api-host 0.0.0.0",
						Times:  1,
						Output: tt.nodeListOutput,
						Error:  nil,
					},
				},
			}

			// Create mock SSH config with behavior
			mockSSH := sshutils.NewMockSSHConfigWithBehavior(behavior)

			err := s.deployer.VerifyBacalhauDeployment(s.ctx, mockSSH, "0.0.0.0")

			if tt.expectError {
				s.Error(err)
				switch tt.name {
				case "Empty node list":
					s.Contains(err.Error(), "no Bacalhau nodes found")
				case "Invalid JSON":
					s.Contains(err.Error(), "failed to strip and parse JSON")
				}
			} else {
				s.NoError(err)
			}

			// Verify expectations
			mockSSH.(*ssh_mock.MockSSHConfiger).AssertExpectations(s.T())
		})
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestFindOrchestratorMachine() {
	tests := []struct {
		name        string
		machines    map[string]models.Machiner
		expectError bool
	}{
		{
			name: "Single orchestrator",
			machines: map[string]models.Machiner{
				"orch": &models.Machine{
					Name:         "orch",
					Orchestrator: true,
					NodeType:     models.BacalhauNodeTypeOrchestrator,
				},
			},
			expectError: false,
		},
		{
			name: "No orchestrator",
			machines: map[string]models.Machiner{
				"worker": &models.Machine{
					Name:     "worker",
					NodeType: models.BacalhauNodeTypeCompute,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.deployment.SetMachines(tt.machines)
			machine, err := s.deployer.FindOrchestratorMachine()

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.True(machine.IsOrchestrator())
			}
		})
	}
}

func TestDeployBacalhauSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAzureDeployBacalhauTestSuite))
}
