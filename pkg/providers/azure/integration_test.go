package azure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/sync/errgroup"
)

type testSetup struct {
	provider        *AzureProvider
	clusterDeployer *common.ClusterDeployer
	mockAzureClient *MockAzureClient
	mockPoller      *MockPoller
	mockSSHConfig   *sshutils.MockSSHConfig
	mockSSHClient   *sshutils.MockSSHClient
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

	mockPoller := new(MockPoller)
	mockPoller.On("PollUntilDone", mock.Anything, mock.Anything).
		Return(armresources.DeploymentsClientCreateOrUpdateResponse{
			DeploymentExtended: testdata.FakeDeployment(),
		}, nil)
	mockAzureClient := new(MockAzureClient)
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

	mockSSHConfig := new(sshutils.MockSSHConfig)

	provider := &AzureProvider{
		Client: mockAzureClient,
	}

	deployment, err := models.NewDeployment()
	assert.NoError(t, err)

	display.SetGlobalModel(display.InitialModel(deployment))
	m := display.GetGlobalModelFunc()
	m.Deployment.Machines = map[string]*models.Machine{
		"orchestrator": {
			Name:         "orchestrator",
			Orchestrator: true,
			PublicIP:     "1.2.3.4",
			Location:     "eastus",
		},
		"worker1": {Name: "worker1", PublicIP: "1.2.3.5", Location: "eastus2"},
		"worker2": {Name: "worker2", PublicIP: "1.2.3.6", Location: "westus"},
	}
	m.Deployment.Azure.ResourceGroupLocation = "eastus"
	m.Deployment.Locations = []string{"eastus", "eastus2", "westus"}

	for _, machine := range m.Deployment.Machines {
		machine.SetServiceState("SSH", models.ServiceStateNotStarted)
		machine.SetServiceState("Docker", models.ServiceStateNotStarted)
		machine.SetServiceState("Bacalhau", models.ServiceStateNotStarted)
	}

	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	cleanup := func() {
		os.Remove(tempConfigFile.Name())
		viper.Reset()
	}

	clusterDeployer := common.NewClusterDeployer()

	return &testSetup{
		provider:        provider,
		clusterDeployer: clusterDeployer,
		mockAzureClient: mockAzureClient,
		mockSSHConfig:   mockSSHConfig,
		cleanup:         cleanup,
	}
}

func setupMockDeployment(mockAzureClient *MockAzureClient) *MockPoller {
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

func setupMockVMAndNetwork(mockAzureClient *MockAzureClient) {
	mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeVirtualMachine(), nil)

	mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeNetworkInterface(), nil)

	mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)
}

func TestProvisionResourcesSuccess(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Mock SSH provisioning failure
	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-docker.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-docker.sh").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker run hello-world").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-core-packages.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-core-packages.sh").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/get-node-config-metadata.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/get-node-config-metadata.sh").
		Return("", nil).
		Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-bacalhau.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-bacalhau.sh").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-run-bacalhau.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-run-bacalhau.sh").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("RestartService",
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Times(3)
	setup.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 0.0.0.0",
	).
		Return(`[{"id": "node1", "public_ip": "1.2.3.4"}]`, nil).
		Times(1)
	setup.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 20.30.40.50",
	).
		Return(`[{"id": "node1", "public_ip": "1.2.3.4"}]`, nil).
		Times(2)

	m := display.GetGlobalModelFunc()
	err := setup.provider.CreateResources(ctx)

	for _, machine := range m.Deployment.Machines {
		err := setup.provider.GetClusterDeployer().ProvisionPackagesOnMachine(ctx, machine.Name)
		assert.NoError(t, err)
	}

	err = setup.provider.GetClusterDeployer().DeployOrchestrator(ctx)
	assert.NoError(t, err)

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}
		err := setup.provider.GetClusterDeployer().DeployWorker(ctx, machine.Name)
		assert.NoError(t, err)
	}

	for _, machine := range m.Deployment.Machines {
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("SSH"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Docker"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
	}

	setup.mockSSHConfig.AssertExpectations(t)
}

func TestSSHProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Mock successful VM deployment
	setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	// Mock SSH provisioning failure
	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("SSH provisioning failed")).Times(3)

	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		machine.SetResourceState(
			models.AzureResourceTypeVM.ResourceString,
			models.ResourceStateSucceeded,
		)
	}

	err := setup.provider.CreateResources(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH provisioning failed")

	// Check that the VM status was updated correctly
	for _, machine := range m.Deployment.Machines {
		assert.Equal(
			t,
			models.ServiceStateFailed,
			machine.GetServiceState("SSH"),
		)
	}

	setup.mockAzureClient.AssertExpectations(t)
	setup.mockSSHConfig.AssertExpectations(t)
}

func TestDockerProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	// Mock SSH provisioning failure
	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-docker.sh",
		mock.Anything,
		mock.Anything).
		Return(fmt.Errorf("fake docker install failure")).Times(3)

	m := display.GetGlobalModelFunc()

	err := setup.provider.CreateResources(ctx)
	assert.NoError(t, err)

	var eg errgroup.Group
	for _, machine := range m.Deployment.Machines {
		eg.Go(func() error {
			return setup.provider.GetClusterDeployer().ProvisionPackagesOnMachine(ctx, machine.Name)
		})
	}

	if err := eg.Wait(); err != nil {
		assert.Error(t, err)
	}

	// Check that the VM status was updated correctly
	for _, machine := range m.Deployment.Machines {
		assert.Equal(
			t,
			models.ServiceStateFailed,
			machine.GetServiceState("Docker"),
		)
	}

	setup.mockAzureClient.AssertExpectations(t)
	setup.mockSSHConfig.AssertExpectations(t)
}

func TestOrchestratorProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	// Mock SSH provisioning failure
	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-docker.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-docker.sh").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker run hello-world").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-core-packages.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(3)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-core-packages.sh").
		Return("", nil).Times(3)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/get-node-config-metadata.sh",
		mock.Anything,
		mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/get-node-config-metadata.sh").
		Return("", nil)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-bacalhau.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(1)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-bacalhau.sh").
		Return("", nil).Times(1)
	setup.mockSSHConfig.On("PushFile",
		mock.Anything,
		"/tmp/install-run-bacalhau.sh",
		mock.Anything,
		mock.Anything).
		Return(nil).Times(1)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-run-bacalhau.sh").
		Return("", nil).Times(1)
	setup.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(1)
	setup.mockSSHConfig.On("RestartService",
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Times(1)
	setup.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 0.0.0.0",
	).
		Return("", fmt.Errorf("bacalhau orchestrator node list failed")).
		Times(1)

	m := display.GetGlobalModelFunc()

	err := setup.provider.CreateResources(ctx)
	assert.NoError(t, err)

	var eg errgroup.Group
	for _, machine := range m.Deployment.Machines {
		eg.Go(func() error {
			return setup.provider.GetClusterDeployer().ProvisionPackagesOnMachine(ctx, machine.Name)
		})
	}
	if err := eg.Wait(); err != nil {
		assert.NoError(t, err)
	}

	err = setup.provider.GetClusterDeployer().DeployOrchestrator(ctx)
	assert.Error(t, err)

	// Check that the VM status was updated correctly
	for _, machine := range m.Deployment.Machines {
		if machine.Name == "orchestrator" {
			assert.Equal(
				t,
				models.ServiceStateFailed,
				machine.GetServiceState("Bacalhau"),
			)
		} else {
			assert.Equal(
				t,
				models.ServiceStateNotStarted,
				machine.GetServiceState("Bacalhau"),
			)
		}
	}

	setup.mockAzureClient.AssertExpectations(t)
	setup.mockSSHConfig.AssertExpectations(t)

}

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
	return args.Get(0).(string), args.Error(1)
}

func (m *MockPoller) Result(
	ctx context.Context,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) Done() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

func (m *MockPoller) Poll(ctx context.Context) (*http.Response, error) {
	args := m.Called(ctx)
	return args.Get(0).(*http.Response), args.Error(1)
}
