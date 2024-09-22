package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/bacalhau-project/andaime/cmd"
	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/gcp"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/azure"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"

	azure_mock "github.com/bacalhau-project/andaime/mocks/azure"
	common_mock "github.com/bacalhau-project/andaime/mocks/common"
	gcp_mock "github.com/bacalhau-project/andaime/mocks/gcp"
)

func setupTestEnvironment(_ *testing.T) (func(), string, string) {
	os.Setenv("ANDAIME_TEST_MODE", "true")
	testSSHPublicKeyPath, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	cleanup := func() {
		os.Unsetenv("ANDAIME_TEST_MODE")
		cleanupPublicKey()
		cleanupPrivateKey()
	}

	return cleanup, testSSHPublicKeyPath, testSSHPrivateKeyPath
}

func setupViper(
	t *testing.T,
	provider models.DeploymentType,
	testSSHPublicKeyPath, testSSHPrivateKeyPath string,
) {
	viper.Reset()
	viper.Set("general.project_prefix", "test-1292")
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)

	tmpConfigFile, err := os.CreateTemp("", "config_*.yaml")
	assert.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpConfigFile.Name()) })

	viper.SetConfigFile(tmpConfigFile.Name())
	assert.NoError(t, viper.ReadInConfig())

	if provider == models.DeploymentTypeAzure {
		viper.Set("azure.subscription_id", "4a45a76b-5754-461d-84a1-f5e47b0a7198")
		viper.Set("azure.default_count_per_zone", 1)
		viper.Set("azure.default_location", "eastus2")
		viper.Set("azure.default_machine_type", "Standard_DS4_v2")
		viper.Set("azure.resource_group_location", "eastus2")
		viper.Set("azure.resource_group_name", "test-1292-rg")
		viper.Set("azure.default_disk_size_gb", 30)
		viper.Set("azure.machines", []interface{}{
			map[string]interface{}{
				"location": "eastus",
			},
			map[string]interface{}{
				"location": "eastus2",
				"parameters": map[string]interface{}{
					"count": float64(2),
				},
			},
			map[string]interface{}{
				"location": "westus",
				"parameters": map[string]interface{}{
					"type": "Standard_DS4_v4",
				},
			},
			map[string]interface{}{
				"location": "westus2",
				"parameters": map[string]interface{}{
					"type": "Standard_D8s_v3",
				},
			},
			map[string]interface{}{
				"location": "centralus",
				"parameters": map[string]interface{}{
					"orchestrator": true,
				},
			},
		})
	} else if provider == models.DeploymentTypeGCP {
		viper.Set("gcp.project_id", "test-1292-gcp")
		viper.Set("gcp.organization_id", "org-1234567890")
		viper.Set("gcp.billing_account_id", "123456-789012-345678")
		viper.Set("gcp.default_count_per_zone", 1)
		viper.Set("gcp.default_location", "us-central1-a")
		viper.Set("gcp.default_machine_type", "n2-standard-2")
		viper.Set("gcp.default_disk_size_gb", 30)
		viper.Set("gcp.machines", []interface{}{
			map[string]interface{}{
				"location": "us-central1-a",
			},
			map[string]interface{}{
				"location": "us-central1-b",
				"parameters": map[string]interface{}{
					"count": float64(2),
				},
			},
			map[string]interface{}{
				"location": "us-central1-c",
				"parameters": map[string]interface{}{
					"type": "n2-standard-4",
				},
			},
			map[string]interface{}{
				"location": "us-central1-f",
				"parameters": map[string]interface{}{
					"type":              "n2-standard-8",
					"disk_image_family": "ubuntu-2204-lts",
				},
			},
			map[string]interface{}{
				"location": "us-central1-g",
				"parameters": map[string]interface{}{
					"orchestrator": true,
				},
			},
		})
	}
}

type MockAzureProvider struct {
	mock.Mock
}

func (m *MockAzureProvider) CreateDeployment(
	ctx context.Context,
	deployment *models.Deployment,
) error {
	return m.Called(ctx, deployment).Error(0)
}

func (m *MockAzureProvider) CreateResources(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockAzureProvider) DestroyResources(ctx context.Context, deploymentName string) error {
	return m.Called(ctx, deploymentName).Error(0)
}

func (m *MockAzureProvider) FinalizeDeployment(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockAzureProvider) GetAzureClient() azure_interface.AzureClienter {
	return m.Called().Get(0).(azure_interface.AzureClienter)
}

func (m *MockAzureProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return m.Called().Get(0).(common_interface.ClusterDeployerer)
}

func (m *MockAzureProvider) GetConfig() *viper.Viper {
	return m.Called().Get(0).(*viper.Viper)
}

func (m *MockAzureProvider) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	return m.Called(ctx, vmName, locationData).Get(0).(string), m.Called(ctx, vmName, locationData).
		Error(1)
}

func (m *MockAzureProvider) GetVMInternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	return m.Called(ctx, vmName, locationData).Get(0).(string), m.Called(ctx, vmName, locationData).
		Error(1)
}

func (m *MockAzureProvider) PollResources(
	ctx context.Context,
) ([]interface{}, error) {
	return m.Called(ctx).Get(0).([]interface{}), m.Called(ctx).Error(1)
}

func (m *MockAzureProvider) PrepareResourceGroup(
	ctx context.Context,
) error {
	return m.Called(ctx).Error(0)
}

func (m *MockAzureProvider) SetAzureClient(client azure_interface.AzureClienter) {
	m.Called(client).Error(0)
}

func (m *MockAzureProvider) SetClusterDeployer(clusterDeployer common_interface.ClusterDeployerer) {
	m.Called(clusterDeployer).Error(0)
}

func (m *MockAzureProvider) SetConfig(config *viper.Viper) {
	m.Called(config).Error(0)
}

func (m *MockAzureProvider) SetSSHClient(sshClient sshutils.SSHClienter) {
	m.Called(sshClient).Error(0)
}

func (m *MockAzureProvider) StartResourcePolling(ctx context.Context) {
	m.Called(ctx).Error(0)
}

func (m *MockAzureProvider) PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	return m.Called(ctx).Get(0).(*models.Deployment), m.Called(ctx).Error(1)
}

func TestExecuteCreateDeployment(t *testing.T) {
	cleanup, testSSHPublicKeyPath, testSSHPrivateKeyPath := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name     string
		provider models.DeploymentType
	}{
		{"Azure", models.DeploymentTypeAzure},
		{"GCP", models.DeploymentTypeGCP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViper(t, tt.provider, testSSHPublicKeyPath, testSSHPrivateKeyPath)

			cmd := cmd.SetupRootCommand()
			cmd.SetContext(context.Background())

			mockClusterDeployer := new(common_mock.MockClusterDeployerer)
			mockClusterDeployer.On("CreateResources", mock.Anything).Return(nil)
			mockClusterDeployer.On("ProvisionMachines", mock.Anything).Return(nil)
			mockClusterDeployer.On("ProvisionBacalhauCluster", mock.Anything).Return(nil)
			mockClusterDeployer.On("FinalizeDeployment", mock.Anything).Return(nil)
			mockClusterDeployer.On("ProvisionOrchestrator", mock.Anything, mock.Anything).
				Return(nil)
			mockClusterDeployer.On("ProvisionWorker", mock.Anything, mock.Anything).Return(nil)
			mockClusterDeployer.On("ProvisionPackagesOnMachine", mock.Anything, mock.Anything).
				Return(nil)
			mockClusterDeployer.On("WaitForAllMachinesToReachState", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			mockClusterDeployer.On("ProvisionBacalhau", mock.Anything).Return(nil)
			mockClusterDeployer.On("PrepareDeployment", mock.Anything, mock.Anything).
				Return(func(ctx context.Context, deploymentType models.DeploymentType) (*models.Deployment, error) {
					return &models.Deployment{DeploymentType: deploymentType}, nil
				})
			mockClusterDeployer.On("GetConfig").Return(viper.New())
			mockClusterDeployer.On("SetConfig", mock.Anything).Return(nil)

			var err error
			var deployment *models.Deployment
			if tt.provider == models.DeploymentTypeAzure {
				viper.Set("azure.subscription_id", "4a45a76b-5754-461d-84a1-f5e47b0a7198")

				mockAzureClient := new(azure_mock.MockAzureClienter)
				mockAzureClient.On("ValidateMachineType",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).
					Return(true, nil)

				// Create a mock provider
				mockAzureProvider := new(MockAzureProvider)
				mockAzureProvider.On("StartResourcePolling", mock.Anything).Return(nil)
				mockAzureProvider.On("PrepareResourceGroup", mock.Anything).
					Return(nil)
				mockAzureProvider.On("CreateResources", mock.Anything).Return(nil)
				mockAzureProvider.On("GetClusterDeployer", mock.Anything).
					Return(mockClusterDeployer)
				mockAzureProvider.On("FinalizeDeployment", mock.Anything).
					Return(nil)

				stringMatcher := mock.MatchedBy(func(s string) bool {
					return strings.Contains(
						s,
						"bacalhau node list --output json --api-host",
					)
				})
				mockSSHConfig := new(sshutils.MockSSHConfig)
				mockSSHConfig.On("ExecuteCommand",
					mock.Anything,
					stringMatcher,
				).
					Return(`[{"id": "node1", "public_ip": "1.2.3.4"}]`, nil)
				mockSSHConfig.On("PushFile",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything).
					Return(nil)
				mockSSHConfig.On("ExecuteCommand",
					mock.Anything,
					mock.Anything).Return(
					"",
					nil,
				)
				mockSSHConfig.On("InstallSystemdService",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil)
				mockSSHConfig.On("RestartService",
					mock.Anything,
					mock.Anything,
				).Return(nil)

				sshutils.NewSSHConfigFunc = func(
					host string,
					port int,
					user string,
					sshPrivateKeyPath string,
				) (sshutils.SSHConfiger, error) {
					return mockSSHConfig, nil
				}
				t.Cleanup(func() {
					sshutils.NewSSHConfigFunc = sshutils.NewSSHConfig
				})

				err = azure.ExecuteCreateDeployment(cmd, []string{})
				if err != nil {
					t.Fatalf("ExecuteCreateDeployment failed: %v", err)
				}
				m := display.GetGlobalModelFunc()
				assert.NotNil(t, m)

				deployment = m.Deployment

				// Assert that CreateDeployment was called
				mockAzureProvider.AssertExpectations(t)

				// Ensure the subscription ID is set correctly
				assert.Equal(
					t,
					"4a45a76b-5754-461d-84a1-f5e47b0a7198",
					deployment.Azure.SubscriptionID,
				)
			} else if tt.provider == models.DeploymentTypeGCP {
				viper.Set("gcp.project_id", "test-1292-gcp")
				viper.Set("gcp.organization_id", "org-1234567890")
				viper.Set("gcp.default_region", "default_region")

				mockGCPClient := new(gcp_mock.MockGCPClienter)
				mockGCPClient.On("ValidateMachineType",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).
					Return(true, nil)

				// Initialize deployment with at least one machine
				m := display.GetGlobalModelFunc()
				m.Deployment, _ = models.NewDeployment()
				machine, _ := models.NewMachine(models.DeploymentTypeAzure, "eastus", "Standard_D2s_v3", 30, models.CloudSpecificInfo{})
				m.Deployment.SetMachines(map[string]models.Machiner{"test-machine": machine})

				err = azure.ExecuteCreateDeployment(cmd, []string{})

				m := display.GetGlobalModelFunc()
				assert.NotNil(t, m)

				deployment = m.Deployment
			}

			if err != nil {
				t.Fatalf("ExecuteCreateDeployment failed: %v", err)
			}

			assert.NotEmpty(t, deployment.Name)
			assert.Equal(
				t,
				strings.ToLower(string(tt.provider)),
				strings.ToLower(string(deployment.DeploymentType)),
			)
			assert.NotEmpty(t, deployment.Machines)

			// Check if SSH keys were properly set
			assert.Equal(t, testSSHPublicKeyPath, deployment.SSHPublicKeyPath)
			assert.Equal(t, testSSHPrivateKeyPath, deployment.SSHPrivateKeyPath)

			// Check provider-specific configurations
			if tt.provider == models.DeploymentTypeAzure {
				assert.Equal(
					t,
					"4a45a76b-5754-461d-84a1-f5e47b0a7198",
					deployment.Azure.SubscriptionID,
				)
				assert.Equal(t, "test-1292-rg", deployment.Azure.ResourceGroupName)
			} else {
				assert.Equal(t, "test-1292-gcp", deployment.GCP.ProjectID, "Project ID should be set correctly")
				assert.Equal(t, "org-1234567890", deployment.GCP.OrganizationID, "Organization ID should be set correctly")
			}
		})
	}
}

func TestPrepareDeployment(t *testing.T) {
	cleanup, testSSHPublicKeyPath, testSSHPrivateKeyPath := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name     string
		provider models.DeploymentType
	}{
		{"Azure", models.DeploymentTypeAzure},
		{"GCP", models.DeploymentTypeGCP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViper(t, tt.provider, testSSHPublicKeyPath, testSSHPrivateKeyPath)

			ctx := context.Background()

			deployment, err := common.PrepareDeployment(ctx, tt.provider)
			if err != nil {
				t.Logf("PrepareDeployment error: %v", err)
			}
			assert.NoError(t, err, "PrepareDeployment failed")
			assert.NotNil(t, deployment)

			assert.NotEmpty(t, deployment.Name)
			assert.Equal(t, tt.provider, deployment.DeploymentType)
			assert.Equal(t, testSSHPublicKeyPath, deployment.SSHPublicKeyPath)
			assert.Equal(t, testSSHPrivateKeyPath, deployment.SSHPrivateKeyPath)
			assert.NotEmpty(t, deployment.SSHPublicKeyMaterial)
			assert.NotEmpty(t, deployment.SSHPrivateKeyMaterial)
			assert.Equal(t, viper.GetString("general.ssh_user"), deployment.SSHUser)

			// Check if SSH port is set correctly - even though the default is empty
			assert.Equal(t, 22, deployment.SSHPort)

			// Check provider-specific configurations
			if tt.provider == models.DeploymentTypeAzure {
				assert.Equal(
					t,
					viper.GetString("azure.subscription_id"),
					deployment.Azure.SubscriptionID,
				)
				assert.Equal(
					t,
					viper.GetString("azure.resource_group_name"),
					deployment.Azure.ResourceGroupName,
				)
				assert.Equal(
					t,
					viper.GetString("azure.resource_group_location"),
					deployment.Azure.ResourceGroupLocation,
				)
				assert.Equal(
					t,
					int32(viper.GetInt("azure.default_disk_size_gb")),
					deployment.Azure.DefaultDiskSizeGB,
				)
				assert.Equal(
					t,
					viper.GetString("azure.default_location"),
					deployment.Azure.DefaultLocation,
				)
				assert.Equal(
					t,
					viper.GetString("azure.default_machine_type"),
					deployment.Azure.DefaultVMSize,
					"Default machine type should be set correctly",
				)
			} else {
				assert.Equal(t, viper.GetString("gcp.project_id"), deployment.GCP.ProjectID)
				assert.Equal(t, viper.GetString("gcp.organization_id"), deployment.GCP.OrganizationID)
				assert.Equal(t, viper.GetString("gcp.billing_account_id"), deployment.GCP.BillingAccountID)
				assert.Equal(t, viper.GetString("gcp.default_region"), deployment.GCP.DefaultRegion)
				assert.Equal(t, viper.GetString("gcp.default_zone"), deployment.GCP.DefaultZone)
				assert.Equal(t, viper.GetString("gcp.default_machine_type"), deployment.GCP.DefaultMachineType)
			}

			// Check if machines are created
			assert.NotEmpty(t, deployment.Machines)

			// Verify machine configurations
			machineConfigsRaw := viper.Get(
				fmt.Sprintf("%s.machines", strings.ToLower(string(tt.provider))),
			)
			machineConfigs, ok := machineConfigsRaw.([]interface{})
			assert.True(t, ok, "Machine configs should be a slice of interfaces")

			expectedMachineCount := 0
			for _, machineConfigRaw := range machineConfigs {
				machineConfig, ok := machineConfigRaw.(map[string]interface{})
				assert.True(t, ok, "Each machine config should be a map")
				if params, ok := machineConfig["parameters"].(map[string]interface{}); ok {
					if count, ok := params["count"]; ok {
						if countFloat, ok := count.(float64); ok {
							expectedMachineCount += int(countFloat)
						} else if countStr, ok := count.(string); ok {
							if countInt, err := strconv.Atoi(countStr); err == nil {
								expectedMachineCount += countInt
							} else {
								expectedMachineCount++
							}
						} else {
							expectedMachineCount++
						}
					} else {
						expectedMachineCount++
					}
				} else {
					expectedMachineCount++
				}
			}
			assert.Equal(t, expectedMachineCount, len(deployment.Machines))

			for _, machine := range deployment.GetMachines() {
				assert.NotEmpty(t, machine.GetName())
				assert.NotEmpty(t, machine.GetLocation())
				if tt.provider == models.DeploymentTypeAzure {
					assert.Contains(
						t,
						[]string{"eastus", "eastus2", "westus", "westus2", "centralus"},
						machine.GetLocation(),
					)
					assert.Contains(
						t,
						[]string{"Standard_DS4_v2", "Standard_DS4_v4", "Standard_D8s_v3"},
						machine.GetVMSize(),
					)
				} else {
					assert.Contains(t, []string{"us-central1-a", "us-central1-b", "us-central1-c", "us-central1-f", "us-central1-g"}, machine.GetLocation())
					assert.Equal(t, "n2-standard-2", machine.GetVMSize(), "Machine VM size should be set correctly")
				}
			}

			// Check for orchestrator
			orchestratorCount := 0
			for _, machine := range deployment.GetMachines() {
				if machine.IsOrchestrator() {
					orchestratorCount++
				}
			}
			assert.Equal(t, 1, orchestratorCount, "There should be exactly one orchestrator")
		})
	}
}
