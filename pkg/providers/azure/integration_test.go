package azure

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/testdata"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type testSetup struct {
	provider        *AzureProvider
	clusterDeployer common_interface.ClusterDeployerer
	mockAzureClient *azure_mocks.MockAzureClienter
	mockPoller      *MockPoller
	cleanup         func()
}

func setupTest(t *testing.T) *testSetup {
	tempConfigFile, err := os.CreateTemp("", "config*.yaml")
	assert.NoError(t, err)

	testConfig, err := testdata.ReadTestAzureConfig()
	assert.NoError(t, err)

	_, err = tempConfigFile.Write([]byte(testConfig))
	assert.NoError(t, err)

	viper.SetConfigFile(tempConfigFile.Name())
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	// Set additional required configuration
	viper.Set("azure.subscription_id", "test-subscription-id")
	viper.Set("azure.resource_group_name", "test-resource-group")
	viper.Set("azure.resource_group_location", "eastus")
	viper.Set("general.ssh_user", "testuser")
	viper.Set("general.ssh_port", 22)
	viper.Set("azure.default_disk_size_gb", 30)

	mockPoller := new(MockPoller)
	mockPoller.On("PollUntilDone", mock.Anything, mock.Anything).
		Return(armresources.DeploymentsClientCreateOrUpdateResponse{
			DeploymentExtended: testdata.FakeDeployment(),
		}, nil)
	mockAzureClient := new(azure_mocks.MockAzureClienter)
	mockAzureClient.On("DeployTemplate",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(mockPoller, nil)

	mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeVirtualMachine(), nil)

	mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeNetworkInterface(), nil)
	mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)

	provider := &AzureProvider{
		Client: mockAzureClient,
	}

	deployment, err := models.NewDeployment()
	assert.NoError(t, err)
	m := display.NewDisplayModel(deployment)

	orchestrator, err := models.NewMachine(
		models.DeploymentTypeAzure,
		"eastus",
		"Standard_D2s_v3",
		30,
		models.CloudSpecificInfo{},
	)
	assert.NoError(t, err)
	orchestrator.SetName("orchestrator")
	orchestrator.SetOrchestrator(true)

	worker1, err := models.NewMachine(
		models.DeploymentTypeAzure,
		"eastus2",
		"Standard_D2s_v3",
		30,
		models.CloudSpecificInfo{},
	)
	assert.NoError(t, err)
	worker1.SetName("worker1")

	worker2, err := models.NewMachine(
		models.DeploymentTypeAzure,
		"westus",
		"Standard_D2s_v3",
		30,
		models.CloudSpecificInfo{},
	)
	assert.NoError(t, err)
	worker2.SetName("worker2")

	m.Deployment.SetMachines(map[string]models.Machiner{
		"orchestrator": orchestrator,
		"worker1":      worker1,
		"worker2":      worker2,
	})
	m.Deployment.Azure.ResourceGroupLocation = "eastus"
	m.Deployment.Locations = []string{"eastus", "eastus2", "westus"}

	for _, machine := range m.Deployment.Machines {
		machine.SetServiceState("SSH", models.ServiceStateNotStarted)
		machine.SetServiceState("Docker", models.ServiceStateNotStarted)
		machine.SetServiceState("Bacalhau", models.ServiceStateNotStarted)
	}

	cleanup := func() {
		os.Remove(tempConfigFile.Name())
		viper.Reset()
	}

	clusterDeployer := common.NewClusterDeployer(models.DeploymentTypeAzure)

	return &testSetup{
		provider:        provider,
		clusterDeployer: clusterDeployer,
		mockAzureClient: mockAzureClient,
		cleanup:         cleanup,
	}
}

func setupMockDeployment(mockAzureClient *azure_mocks.MockAzureClienter) *MockPoller {
	props := armresources.DeploymentsClientCreateOrUpdateResponse{}
	successState := armresources.ProvisioningStateSucceeded
	props.Properties = &armresources.DeploymentPropertiesExtended{
		ProvisioningState: &successState,
	}
	mockArmDeploymentPoller := &MockPoller{}
	mockArmDeploymentPoller.On("PollUntilDone", mock.Anything, mock.Anything).Return(props, nil)
	mockAzureClient.On("DeployTemplate",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(mockArmDeploymentPoller, nil)
	return mockArmDeploymentPoller
}

func setupMockVMAndNetwork(mockAzureClient *azure_mocks.MockAzureClienter) {
	mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeVirtualMachine(), nil)

	mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeNetworkInterface(), nil)

	mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)
}

func TestProvisionResourcesSuccess(t *testing.T) {
	l := logger.Get()
	l.Info("Starting TestProvisionResourcesSuccess")

	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Initialize the global display model
	deployment, err := models.NewDeployment()
	assert.NoError(t, err)
	m := display.NewDisplayModel(deployment)
	display.SetGlobalModel(m)

	// Define ExpectedSSHBehavior
	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/install-docker.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-core-packages.sh",
				FileContents:     []byte(""),
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
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
				Cmd:              "sudo /tmp/install-docker.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo docker run hello-world",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-core-packages.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/get-node-config-metadata.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-bacalhau.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-run-bacalhau.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				CmdMatcher: func(cmd string) bool {
					return cmd == "bacalhau node list --output json --api-host 0.0.0.0" ||
						cmd == "bacalhau node list --output json --api-host 20.30.40.50"
				},
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1", "public_ip": "1.2.3.4"}]`,
				Error:            nil,
				Times:            3,
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

	// Create mockSSHConfig
	mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(sshBehavior)
	// Set up WaitForSSH expectations
	mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(3)

	// Override sshutils.NewSSHConfigFunc
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	err = setup.provider.CreateResources(ctx)
	setup.provider.SetClusterDeployer(setup.clusterDeployer)

	for _, machine := range m.Deployment.Machines {
		err := setup.provider.GetClusterDeployer().
			ProvisionPackagesOnMachine(ctx, machine.GetName())
		assert.NoError(t, err)
	}

	err = setup.provider.GetClusterDeployer().ProvisionOrchestrator(ctx, "orchestrator")
	assert.NoError(t, err)

	for _, machine := range m.Deployment.Machines {
		if machine.IsOrchestrator() {
			continue
		}
		err := setup.provider.GetClusterDeployer().ProvisionWorker(ctx, machine.GetName())
		assert.NoError(t, err)
	}

	for _, machine := range m.Deployment.Machines {
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("SSH"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Docker"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
	}

	mockSSHConfig.AssertExpectations(t)
}

// Similar updates should be made to the other test functions like TestSSHProvisioningFailure, TestDockerProvisioningFailure, and TestOrchestratorProvisioningFailure.

type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) PollUntilDone(
	ctx context.Context,
	options *runtime.PollUntilDoneOptions,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) ResumeToken() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockPoller) Result(
	ctx context.Context,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) Done() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockPoller) Poll(ctx context.Context) (*http.Response, error) {
	args := m.Called(ctx)
	return args.Get(0).(*http.Response), args.Error(1)
}
