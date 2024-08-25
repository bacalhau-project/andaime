package azure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type testSetup struct {
	provider        *AzureProvider
	mockAzureClient *MockAzureClient
	mockSSHConfig   *MockSSHConfig
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

	mockAzureClient := new(MockAzureClient)
	mockSSHConfig := new(MockSSHConfig)
	mockSSHClient := new(sshutils.MockSSHClient)
	mockSSHClient.Session = new(sshutils.MockSSHSession)

	provider := &AzureProvider{
		Client: mockAzureClient,
	}

	display.SetGlobalModel(display.InitialModel())
	m := display.GetGlobalModel()
	m.Deployment = models.NewDeployment()
	m.Deployment.Machines = map[string]*models.Machine{
		"orchestrator": {Name: "orchestrator", Orchestrator: true, PublicIP: "1.2.3.4"},
		"worker1":      {Name: "worker1", PublicIP: "1.2.3.5"},
		"worker2":      {Name: "worker2", PublicIP: "1.2.3.6"},
	}
	m.Deployment.ResourceGroupLocation = "eastus"

	sshutils.NewSSHConfigFunc = func(host string, port int, user string, privateKeyMaterial []byte) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	cleanup := func() {
		os.Remove(tempConfigFile.Name())
		viper.Reset()
	}

	return &testSetup{
		provider:        provider,
		mockAzureClient: mockAzureClient,
		mockSSHConfig:   mockSSHConfig,
		mockSSHClient:   mockSSHClient,
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
	mockAzureClient.On("DeployTemplate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(mockArmDeploymentPoller, nil)
	return mockArmDeploymentPoller
}

func setupMockVMAndNetwork(mockAzureClient *MockAzureClient) {
	nicID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkInterfaces/nic1"
	mockVM := &armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{{ID: &nicID}},
			},
		},
	}
	mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
		Return(mockVM, nil)

	privateIPAddress := "10.0.0.4"
	publicIPAddressID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/publicIPAddresses/pip1"
	mockNIC := &armnetwork.Interface{
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{{
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
					PrivateIPAddress: &privateIPAddress,
					PublicIPAddress:  &armnetwork.PublicIPAddress{ID: &publicIPAddressID},
				},
			}},
		},
	}
	mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
		Return(mockNIC, nil)

	publicIPAddress := "20.30.40.50"
	mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
		Return(publicIPAddress, nil)
}

func TestPrepareDeployment(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	setup.mockAzureClient.On("GetOrCreateResourceGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&armresources.ResourceGroup{}, nil)

	err := setup.provider.PrepareDeployment(ctx)
	assert.NoError(t, err)

	setup.mockAzureClient.AssertExpectations(t)
}

func TestProvisionResourcesSuccess(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	fakeOrchestratorIP := "20.30.40.50"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mockArmDeploymentPoller := setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker version -f json").
		Return(`{"Client":{"Version":"1.2.3"},"Server":{"Version":"1.2.3"}}`, nil)
	setup.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("RestartService", mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(command string) bool {
		return command == fmt.Sprintf(
			"bacalhau node list --output json --api-host %s",
			fakeOrchestratorIP,
		) ||
			command == "bacalhau node list --output json --api-host 0.0.0.0"
	})).
		Return(`[{"id": "node1"}]`, nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)

	err := setup.provider.ProvisionResources(ctx)
	assert.NoError(t, err)

	m := display.GetGlobalModel()
	for _, machine := range m.Deployment.Machines {
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("SSH"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Docker"))
		assert.Equal(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
		assert.Equal(t, fakeOrchestratorIP, machine.PublicIP)
		assert.Equal(t, "10.0.0.4", machine.PrivateIP)
	}

	mockArmDeploymentPoller.AssertExpectations(t)
	setup.mockAzureClient.AssertExpectations(t)
	setup.mockSSHConfig.AssertExpectations(t)
}

func TestProvisionResourcesFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	setup.mockAzureClient.On("DeployTemplate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&MockPoller{}, fmt.Errorf("resource provisioning failed"))

	err := setup.provider.ProvisionResources(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource provisioning failed")

	setup.mockAzureClient.AssertExpectations(t)
}

func TestSSHProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mockArmDeploymentPoller := setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything).
		Return(fmt.Errorf("SSH provisioning failed"))

	err := setup.provider.ProvisionResources(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH provisioning failed")

	mockArmDeploymentPoller.AssertExpectations(t)
	setup.mockAzureClient.AssertExpectations(t)
	setup.mockSSHConfig.AssertExpectations(t)
}

func TestDockerProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mockArmDeploymentPoller := setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker version -f json").
		Return(`{"Client":{"Version":"1.2.3"}}`, nil) // Missing Server version
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)

	err := setup.provider.ProvisionResources(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Docker server version")

	mockArmDeploymentPoller.AssertExpectations(t)
	setup.mockAzureClient.AssertExpectations(t)
	setup.mockSSHConfig.AssertExpectations(t)
}

func TestOrchestratorProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mockArmDeploymentPoller := setupMockDeployment(setup.mockAzureClient)
	setupMockVMAndNetwork(setup.mockAzureClient)

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker version -f json").
		Return(`{"Client":{"Version":"1.2.3"},"Server":{"Version":"1.2.3"}}`, nil)
	setup.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("RestartService", mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "bacalhau node list --output json --api-host 0.0.0.0").
		Return(`[]`, nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)

	err := setup.provider.ProvisionResources(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Bacalhau nodes found")

	m := display.GetGlobalModel()
	for _, machine := range m.Deployment.Machines {
		if !machine.Orchestrator {
			assert.NotEqual(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
		}
	}

	mockArmDeploymentPoller.AssertExpectations(t)
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
