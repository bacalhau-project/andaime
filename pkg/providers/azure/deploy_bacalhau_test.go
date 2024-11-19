package azure

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// TestHelper encapsulates common test setup and verification
type TestHelper struct {
	t            *testing.T
	mockSSH      *sshutils.MockSSHConfig
	expectedCall map[string]int   // tracks expected calls per method
	actualCalls  map[string]int   // tracks actual calls per method
	callArgs     map[string][]any // stores arguments per call for verification
}

func NewTestHelper(t *testing.T, mockSSH *sshutils.MockSSHConfig) *TestHelper {
	return &TestHelper{
		t:            t,
		mockSSH:      mockSSH,
		expectedCall: make(map[string]int),
		actualCalls:  make(map[string]int),
		callArgs:     make(map[string][]any),
	}
}

// ExpectCall registers an expected call with args
func (h *TestHelper) ExpectCall(method string, times int, args ...any) {
	h.expectedCall[method] = times
	h.callArgs[method] = args
}

// OnCall sets up a mock call with verification
func (h *TestHelper) OnCall(method string, args ...any) *mock.Call {
	return h.mockSSH.On(method, args...).Run(func(args mock.Arguments) {
		h.actualCalls[method]++
		h.t.Logf("Called %s (%d/%d times)", method, h.actualCalls[method], h.expectedCall[method])
	})
}

// VerifyCalls checks if all expected calls were made correctly
func (h *TestHelper) VerifyCalls() {
	h.t.Log("\nCall Verification:")
	for method, expected := range h.expectedCall {
		actual := h.actualCalls[method]
		if actual != expected {
			h.t.Errorf("Method %s: Expected %d calls, got %d", method, expected, actual)
		}
	}
}

type PkgProvidersAzureDeployBacalhauTestSuite struct {
	suite.Suite
	ctx        context.Context
	deployment *models.Deployment
	deployer   *common.ClusterDeployer
	testHelper *TestHelper
}

// Add these methods to the TestHelper struct
func (h *TestHelper) Reset() {
	h.expectedCall = make(map[string]int)
	h.actualCalls = make(map[string]int)
	h.callArgs = make(map[string][]any)
	h.mockSSH.ExpectedCalls = nil
	h.mockSSH.Calls = nil
}

// Update the suite's SetupTest method to initialize everything properly
func (s *PkgProvidersAzureDeployBacalhauTestSuite) SetupTest() {
	s.ctx = context.Background()
	viper.Set("general.project_prefix", "test-project")

	// Create a fresh deployment for each test
	var err error
	s.deployment, err = models.NewDeployment()
	s.NoError(err)
	s.deployer = common.NewClusterDeployer(models.DeploymentTypeAzure)

	// Create fresh mocks for each test
	mockSSH := sshutils.NewMockSSHConfigWithBehavior(sshutils.ExpectedSSHBehavior{})
	s.testHelper = NewTestHelper(s.T(), mockSSH)

	// Reset the global SSH config function for each test
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSH, nil
	}

	// Reset the global model function for each test
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: s.deployment,
		}
	}
}

// Add TearDownTest to clean up after each test
func (s *PkgProvidersAzureDeployBacalhauTestSuite) TearDownTest() {
	if s.testHelper != nil {
		s.testHelper.Reset()
	}

	// Clear any global state
	s.deployment = nil
	s.deployer = nil
}

// Example test using the new helper
func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployOrchestrator() {
	s.SetupTest()

	// Setup test data
	s.deployment.SetMachines(map[string]models.Machiner{
		"orch": &models.Machine{
			Name:         "orch",
			Orchestrator: true,
			PublicIP:     "1.2.3.4",
			NodeType:     models.BacalhauNodeTypeOrchestrator,
		},
	})

	// Define expected calls
	s.testHelper.ExpectCall("PushFile", 5)
	s.testHelper.ExpectCall("ExecuteCommand", 7)
	s.testHelper.ExpectCall("InstallSystemdService", 1)
	s.testHelper.ExpectCall("RestartService", 1)

	// Setup mock behaviors with clear logging
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-docker.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-core-packages.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/get-node-config-metadata.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-bacalhau.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-run-bacalhau.sh", mock.Anything, true, mock.Anything).
		Return(nil)

	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo docker run hello-world", mock.Anything).
		Return("Hello from Docker!", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-docker.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-core-packages.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/get-node-config-metadata.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-bacalhau.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-run-bacalhau.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "bacalhau node list --output json --api-host 0.0.0.0", mock.Anything).
		Return(`[{"id": "node1"}]`, nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo bacalhau config list --output json", mock.Anything).
		Return("[]", nil)

	s.testHelper.OnCall("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("RestartService", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.testHelper.OnCall("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Run the test
	err := s.deployer.ProvisionOrchestrator(s.ctx, "orch")
	s.NoError(err)

	// Verify all calls were made as expected
	s.testHelper.VerifyCalls()
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestDeployWorker() {
	s.SetupTest()

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

	// Define expected calls
	s.testHelper.ExpectCall("PushFile", 5) // 5 script pushes
	s.testHelper.ExpectCall("ExecuteCommand", 7)
	s.testHelper.ExpectCall("InstallSystemdService", 1)
	s.testHelper.ExpectCall("RestartService", 1)

	// Setup mock behaviors
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-docker.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-core-packages.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/get-node-config-metadata.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-bacalhau.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-run-bacalhau.sh", mock.Anything, true, mock.Anything).
		Return(nil)

	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo docker run hello-world", mock.Anything).
		Return("Hello from Docker!", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-docker.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-core-packages.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/get-node-config-metadata.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-bacalhau.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-run-bacalhau.sh", mock.Anything).
		Return("", nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "bacalhau node list --output json --api-host 1.2.3.4", mock.Anything).
		Return(`[{"id": "node1"}]`, nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo bacalhau config list --output json", mock.Anything).
		Return("[]", nil)

	s.testHelper.OnCall("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("RestartService", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.testHelper.OnCall("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Run the test
	err := s.deployer.ProvisionWorker(s.ctx, "worker")
	s.NoError(err)

	// Verify all calls were made as expected
	s.testHelper.VerifyCalls()
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestSetupNodeConfigMetadata() {
	s.SetupTest()

	s.deployment.SetMachines(map[string]models.Machiner{
		"test": &models.Machine{
			Name:     "test",
			VMSize:   "Standard_DS4_v2",
			Location: "eastus2",
			NodeType: models.BacalhauNodeTypeOrchestrator,
		},
	})

	// Define expected calls
	s.testHelper.ExpectCall("PushFile", 1)
	s.testHelper.ExpectCall("ExecuteCommand", 1)

	// Setup mock behaviors
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/get-node-config-metadata.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/get-node-config-metadata.sh", mock.Anything).
		Return("", nil)

	err := s.deployer.SetupNodeConfigMetadata(
		s.ctx,
		s.deployment.GetMachine("test"),
		s.testHelper.mockSSH,
	)
	s.NoError(err)

	s.testHelper.VerifyCalls()
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestInstallBacalhau() {
	s.SetupTest()

	// Define expected calls
	s.testHelper.ExpectCall("PushFile", 1)
	s.testHelper.ExpectCall("ExecuteCommand", 1)

	// Setup mock behaviors
	s.testHelper.OnCall("PushFile", mock.Anything, "/tmp/install-bacalhau.sh", mock.Anything, true, mock.Anything).
		Return(nil)
	s.testHelper.OnCall("ExecuteCommand", mock.Anything, "sudo /tmp/install-bacalhau.sh", mock.Anything).
		Return("", nil)

	err := s.deployer.InstallBacalhau(s.ctx, s.testHelper.mockSSH)
	s.NoError(err)

	s.testHelper.VerifyCalls()
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
			s.SetupTest()

			// Define expected calls
			s.testHelper.ExpectCall("ExecuteCommand", 1)

			// Setup mock behavior
			s.testHelper.OnCall("ExecuteCommand", mock.Anything, "bacalhau node list --output json --api-host 0.0.0.0", mock.Anything).
				Return(tt.nodeListOutput, nil)

			err := s.deployer.VerifyBacalhauDeployment(s.ctx, s.testHelper.mockSSH, "0.0.0.0")

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}

			s.testHelper.VerifyCalls()
		})
	}
}

func (s *PkgProvidersAzureDeployBacalhauTestSuite) TestFindOrchestratorMachine() {
	s.SetupTest()

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
