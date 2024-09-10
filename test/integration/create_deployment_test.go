package integration

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/bacalhau-project/andaime/cmd"
	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/gcp"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	tea "github.com/charmbracelet/bubbletea"
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) PrepareResourceGroup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockProvider) CreateResources(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockProvider) FinalizeDeployment(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockProgram struct {
	mock.Mock
}

func (m *MockProgram) Init() tea.Msg {
	args := m.Called()
	return args.Get(0).(tea.Msg)
}

func (m *MockProgram) InitProgram(model *display.DisplayModel) {
	m.Called(model)
}

func (m *MockProgram) Quit() {
	m.Called()
}

func (m *MockProgram) Run() (tea.Model, error) {
	args := m.Called()
	return args.Get(0).(tea.Model), args.Error(1)
}

func (m *MockProgram) GetProgram() *display.GlobalProgram {
	args := m.Called()
	return args.Get(0).(*display.GlobalProgram)
}

type MockPollerer struct {
	mock.Mock
}

func (m *MockPollerer) PollUntilDone(
	ctx context.Context,
	options *runtime.PollUntilDoneOptions,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPollerer) ResumeToken() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockPollerer) Result(
	ctx context.Context,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPollerer) Done() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockPollerer) Poll(ctx context.Context) (*http.Response, error) {
	args := m.Called(ctx)
	return args.Get(0).(*http.Response), args.Error(1)
}

type MockSSHConfig struct {
	mock.Mock
}

func (m *MockSSHConfig) WaitForSSH(ctx context.Context, ip string, port int, user string) error {
	args := m.Called(ctx, ip, port, user)
	return args.Error(0)
}

func TestExecuteCreateDeployment(t *testing.T) {
	// Set the test mode environment variable
	os.Setenv("ANDAIME_TEST_MODE", "true")
	defer os.Unsetenv("ANDAIME_TEST_MODE")

	testSSHPublicKeyPath,
		cleanupPublicKey,
		testSSHPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockSSHConfig := new(sshutils.MockSSHConfig)
	mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(command string) bool {
		return strings.Contains(command, "bacalhau node list --output json --api-host")
	})).
		Return(`[{"id": "node1"}]`, nil)

	mockSSHConfig.On("WaitForSSH", mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	mockSSHConfig.On("PushFile", mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	mockSSHConfig.On("ExecuteCommand", mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("", nil)
	mockSSHConfig.On("InstallSystemdService", mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	mockSSHConfig.On("RestartService", mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}

	cmd := cmd.SetupRootCommand()
	cmd.SetContext(context.Background())
	tests := []struct {
		name     string
		provider string
	}{
		{"Azure", "azure"},
		{"GCP", "gcp"},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("general.project_prefix", "test-1292")
			viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
			viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)

			tmpConfigFile, err := os.CreateTemp("", "config_*.yaml")
			assert.NoError(t, err)
			defer os.Remove(tmpConfigFile.Name())

			viper.SetConfigFile(tmpConfigFile.Name())
			assert.NoError(t, viper.ReadInConfig())

			// Prevent tea program from rendering
			os.Setenv("TEST_MODE", "true")
			defer os.Unsetenv("TEST_MODE")

			// Create a mock DisplayModel that implements tea.Model
			mockDisplayModel := &display.DisplayModel{}

			// Mock tea program
			mockProgram := new(MockProgram)
			mockProgram.On("InitProgram", mock.Anything).Return(nil)
			mockProgram.On("Quit").Return(nil)
			mockProgram.On("Run").Return(mockDisplayModel, nil)

			origNewProgramFunc := display.GetGlobalProgramFunc
			t.Cleanup(func() { display.GetGlobalProgramFunc = origNewProgramFunc })
			display.GetGlobalProgramFunc = func() display.GlobalProgammer {
				return mockProgram
			}

			if tt.provider == "azure" {
				// Generic Resource Group
				rg := &armresources.ResourceGroup{
					Name:     to.Ptr("test-1292-rg"),
					Location: to.Ptr("eastus2"),
					Tags: map[string]*string{
						"test": to.Ptr("test"),
					},
				}
				resp := armresources.DeploymentsClientCreateOrUpdateResponse{
					DeploymentExtended: armresources.DeploymentExtended{
						Properties: &armresources.DeploymentPropertiesExtended{
							ProvisioningState: to.Ptr(armresources.ProvisioningStateSucceeded),
						},
					},
				}
				nicID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkInterfaces/nic1"
				vm := &armcompute.VirtualMachine{
					Properties: &armcompute.VirtualMachineProperties{
						NetworkProfile: &armcompute.NetworkProfile{
							NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
								{ID: &nicID},
							},
						},
					},
				}
				poller := &MockPollerer{}
				poller.On(
					"PollUntilDone",
					mock.Anything,
					mock.Anything,
				).Return(resp, nil)
				mockAzureClient := new(azure_provider.MockAzureClient)
				mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
					Return(true, nil)
				mockAzureClient.On("GetOrCreateResourceGroup",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(rg, nil)
				mockAzureClient.On("DeployTemplate",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(poller, nil)
				mockAzureClient.On("GetVirtualMachine",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(vm, nil)
				privateIPAddress := "10.0.0.4"
				publicIPAddressID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/publicIPAddresses/pip1"
				mockNIC := &armnetwork.Interface{
					Properties: &armnetwork.InterfacePropertiesFormat{
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{{
							Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
								PrivateIPAddress: &privateIPAddress,
								PublicIPAddress: &armnetwork.PublicIPAddress{
									ID: &publicIPAddressID,
								},
							},
						}},
					},
				}
				mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
					Return(mockNIC, nil)
				publicIPAddress := "20.30.40.50"
				mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
					Return(publicIPAddress, nil)

				origNewAzureClientFunc := azure_provider.NewAzureClientFunc
				t.Cleanup(func() { azure_provider.NewAzureClientFunc = origNewAzureClientFunc })
				azure_provider.NewAzureClientFunc = func(subscriptionID string) (azure_provider.AzureClienter, error) {
					return mockAzureClient, nil
				}

				viper.Set("azure.subscription_id", "4a45a76b-5754-461d-84a1-f5e47b0a7198")
				viper.Set("azure.default_count_per_zone", 1)
				viper.Set("azure.default_location", "eastus2")
				viper.Set("azure.default_machine_type", "Standard_DS4_v2")
				viper.Set("azure.resource_group_location", "eastus2")
				viper.Set("azure.resource_group_name", "test-1292-rg")
				viper.Set("azure.default_disk_size_gb", 30)
				viper.Set("azure.machines", []map[string]interface{}{
					{
						"location": "eastus2",
						"parameters": map[string]interface{}{
							"count": 2,
						},
					},
					{
						"location": "westus",
						"parameters": map[string]interface{}{
							"orchestrator": true,
						},
					},
				})
				assert.NoError(t, err)
				mockProgram.On("Run").Return(display.DisplayModel{}, nil)

				err = azure.ExecuteCreateDeployment(cmd, []string{})
				mockAzureClient.AssertExpectations(t)
			} else {
				instances := []*computepb.Instance{
					{
						Name: to.Ptr("test-1292-instance-1"),
						NetworkInterfaces: []*computepb.NetworkInterface{
							{
								AccessConfigs: []*computepb.AccessConfig{
									{
										NatIP: to.Ptr("256.256.256.256"),
									},
								},
								NetworkIP: to.Ptr("10.0.0.4"),
							},
						},
					},
					{
						Name: to.Ptr("test-1292-instance-2"),
						NetworkInterfaces: []*computepb.NetworkInterface{
							{
								AccessConfigs: []*computepb.AccessConfig{
									{
										NatIP: to.Ptr("256.256.256.257"),
									},
								},
								NetworkIP: to.Ptr("10.0.0.5"),
							},
						},
					},
				}

				viper.Set("gcp.project_id", "test-1292-gcp")
				viper.Set("gcp.organization_id", "org-1234567890")
				viper.Set("gcp.billing_account_id", "123456-789012-345678")
				viper.Set("gcp.default_count_per_zone", 1)
				viper.Set("gcp.default_location", "us-central1-a")
				viper.Set("gcp.default_machine_type", "n2-highcpu-4")
				viper.Set("gcp.default_disk_size_gb", 30)
				viper.Set("gcp.machines", []map[string]interface{}{
					{
						"location": "us-central1-a",
						"parameters": map[string]interface{}{
							"count": 2,
						},
					},
					{
						"location": "us-central1-b",
						"parameters": map[string]interface{}{
							"orchestrator": true,
						},
					},
				})

				mockGCPClient := new(gcp_provider.MockGCPClient)
				mockGCPClient.On("EnsureProject", mock.Anything, mock.Anything, mock.Anything).
					Return("test-1292-gcp", nil)
				mockGCPClient.On("IsAPIEnabled", mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything).Return(true, nil)
				mockGCPClient.On("CreateFirewallRules",
					mock.Anything,
					mock.Anything,
				).Return(nil)
				mockGCPClient.On("CreateComputeInstance",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(instances[0], nil)

				origNewGCPClientFunc := gcp_provider.NewGCPClientFunc
				t.Cleanup(func() { gcp_provider.NewGCPClientFunc = origNewGCPClientFunc })
				gcp_provider.NewGCPClientFunc = func(ctx context.Context, organizationID string) (gcp_provider.GCPClienter, func(), error) {
					return mockGCPClient, func() {}, nil
				}
				err = gcp.ExecuteCreateDeployment(cmd, []string{})
			}

			assert.NoError(t, err)
		})
	}
}

func TestPrepareDeployment(t *testing.T) {
	testSSHPublicKeyPath,
		cleanupPublicKey,
		testSSHPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockSSHConfig := new(sshutils.MockSSHConfig)
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return mockSSHConfig, nil
	}
	tests := []struct {
		name     string
		errorMsg string
		provider models.DeploymentType
	}{
		{name: "Azure", errorMsg: "", provider: models.DeploymentTypeAzure},
		{name: "GCP", errorMsg: "", provider: models.DeploymentTypeGCP},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			viper.Reset()

			viper.Set("general.project_prefix", "test-1292")
			viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
			viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)
			viper.Set("general.project_prefix", "test")

			// Create a mock DisplayModel that implements tea.Model
			mockDisplayModel := &display.DisplayModel{}

			// Mock tea program
			mockProgram := new(MockProgram)
			mockProgram.On("InitProgram", mock.Anything).Return(nil)
			mockProgram.On("Quit").Return(nil)
			mockProgram.On("Run").Return(mockDisplayModel, nil)
			tmpConfigFile, err := os.CreateTemp("", "config_*.yaml")
			assert.NoError(t, err)
			defer os.Remove(tmpConfigFile.Name())

			viper.SetConfigFile(tmpConfigFile.Name())
			assert.NoError(t, viper.ReadInConfig())

			// Prevent tea program from rendering
			os.Setenv("ANDAIME_TEST_MODE", "true")
			defer os.Unsetenv("ANDAIME_TEST_MODE")

			var deployment *models.Deployment
			deployment, err = common.PrepareDeployment(ctx, tt.provider)
			m := display.NewDisplayModel(deployment)

			if tt.provider == models.DeploymentTypeAzure {
				viper.Set("azure.subscription_id", "4a45a76b-5754-461d-84a1-f5e47b0a7198")
				viper.Set("azure.default_count_per_zone", 1)
				viper.Set("azure.default_location", "eastus2")
				viper.Set("azure.default_machine_type", "Standard_D2_v2")
				viper.Set("azure.default_disk_size_gb", 30)
				viper.Set("azure.machines", []map[string]interface{}{
					{
						"location": "australiasoutheast",
						"parameters": map[string]interface{}{
							"count": 2,
						},
					},
					{
						"location": "brazilsouth",
						"parameters": map[string]interface{}{
							"orchestrator": true,
						},
					},
				})

				mockAzureClient := new(azure_provider.MockAzureClient)
				mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
					Return(true, nil)
				origNewAzureClientFunc := azure_provider.NewAzureClientFunc
				t.Cleanup(func() { azure_provider.NewAzureClientFunc = origNewAzureClientFunc })
				azure_provider.NewAzureClientFunc = func(subscriptionID string) (azure_provider.AzureClienter, error) {
					return mockAzureClient, nil
				}
			} else if tt.provider == models.DeploymentTypeGCP {
				viper.Set("gcp.project_id", "test-1292-gcp")
				viper.Set("gcp.organization_id", "org-1234567890")
				viper.Set("gcp.billing_account_id", "123456-789012-345678")
				viper.Set("gcp.default_count_per_zone", 1)
				viper.Set("gcp.default_location", "me-central1-a")
				viper.Set("gcp.default_machine_type", "n2-highcpu-4")
				viper.Set("gcp.default_disk_size_gb", 30)
				viper.Set("gcp.machines", []map[string]interface{}{
					{
						"location": "me-central1-a",
						"parameters": map[string]interface{}{
							"count": 4,
						},
					},
					{
						"location": "europe-west12-a",
						"parameters": map[string]interface{}{
							"orchestrator": true,
						},
					},
				})

			}
			err = common.ProcessMachinesConfig(
				tt.provider,
				func(ctx context.Context, location string, machineType string) (bool, error) {
					return true, nil
				},
			)

			assert.NoError(t, err)
			assert.NotNil(t, m.Deployment)
			assert.NotEmpty(t, m.Deployment.Name)
			assert.NotEmpty(t, m.Deployment.Machines)
		})
	}
}
