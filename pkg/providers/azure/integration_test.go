package azure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
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

	provider := &AzureProvider{
		Client: mockAzureClient,
	}

	display.SetGlobalModel(display.InitialModel())
	m := display.GetGlobalModelFunc()
	m.Deployment = models.NewDeployment()
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
	m.Deployment.ResourceGroupLocation = "eastus"
	m.Deployment.Locations = []string{"eastus", "eastus2", "westus"}

	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		privateKeyMaterial []byte) (sshutils.SSHConfiger, error) {
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

func TestProvisionResourcesSuccess(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stringMatcher := func(command string) bool {
		return strings.Contains(command,
			"bacalhau node list --output json --api-host")
	}

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker version -f json").
		Return(`{"Client":{"Version":"1.2.3"},"Server":{"Version":"1.2.3"}}`, nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(stringMatcher)).
		Return(`[{"id": "node1"}]`, nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)
	setup.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("RestartService", mock.Anything, mock.Anything).Return(nil)

	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		machine.SetResourceState(
			models.AzureResourceTypeVM.ResourceString,
			models.AzureResourceStateSucceeded,
		)
	}

	err := setup.provider.ProvisionPackagesOnMachines(ctx)
	assert.NoError(t, err)

	err = setup.provider.ProvisionBacalhau(ctx)
	assert.NoError(t, err)

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

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("SSH provisioning failed"))

	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		machine.SetResourceState(
			models.AzureResourceTypeVM.ResourceString,
			models.AzureResourceStateSucceeded,
		)
	}

	err := setup.provider.ProvisionPackagesOnMachines(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH provisioning failed")

	setup.mockSSHConfig.AssertExpectations(t)
}

func TestDockerProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	setup.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)
	setup.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo /tmp/install-docker.sh").
		Return("", fmt.Errorf("failed to install Docker"))

	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		machine.SetResourceState(
			models.AzureResourceTypeVM.ResourceString,
			models.AzureResourceStateSucceeded,
		)
	}

	err := setup.provider.ProvisionPackagesOnMachines(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal Docker server version")

	setup.mockSSHConfig.AssertExpectations(t)
}

func TestOrchestratorProvisioningFailure(t *testing.T) {
	setup := setupTest(t)
	defer setup.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.Machines {
		machine.SetResourceState(
			models.AzureResourceTypeVM.ResourceString,
			models.AzureResourceStateSucceeded,
		)
	}

	setup.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)
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

	err := setup.provider.ProvisionPackagesOnMachines(ctx)
	if err != nil {
		assert.Fail(t, "error provisioning packages on machines", err)
	}

	err = setup.provider.ProvisionBacalhau(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Bacalhau nodes found")

	for _, machine := range m.Deployment.Machines {
		assert.NotEqual(t, models.ServiceStateSucceeded, machine.GetServiceState("Bacalhau"))
	}
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
