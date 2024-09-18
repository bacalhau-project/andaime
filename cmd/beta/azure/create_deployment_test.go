package azure

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/internal/testutil"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/azure"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AzureProviderTestSuite struct {
	suite.Suite
	ctx                    context.Context
	testSSHPublicKeyPath   string
	testPrivateKeyPath     string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockAzureClient        *azure_mocks.MockAzureClienter
	azureProvider          azure_interface.AzureProviderer
	origGetGlobalModelFunc func() *display.DisplayModel
}

func (suite *AzureProviderTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath, suite.cleanupPublicKey, suite.testPrivateKeyPath, suite.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	suite.mockAzureClient = new(azure_mocks.MockAzureClienter)
	suite.mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		deployment, err := models.NewDeployment()
		suite.Require().NoError(err)
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	_ = logger.Get()
}

func (suite *AzureProviderTestSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
}

func (suite *AzureProviderTestSuite) SetupTest() {
	viper.Reset()
	suite.setupViper()

	var err error
	commonProvider, err := azure_provider.NewAzureProvider(
		suite.ctx,
		viper.GetString("azure.subscription_id"),
	)
	suite.Require().NoError(err)
	suite.azureProvider = commonProvider.(azure_interface.AzureProviderer)
	suite.Require().NotNil(suite.azureProvider)
}

func (suite *AzureProviderTestSuite) setupViper() {
	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	viper.Set("general.ssh_public_key_path", suite.testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", suite.testPrivateKeyPath)
	viper.Set("azure.subscription_id", "test-subscription-id")
	viper.Set("azure.default_count_per_zone", 1)
	viper.Set("azure.default_machine_type", "Standard_DS4_v2")
	viper.Set("azure.default_disk_size_gb", 30)
}

func (suite *AzureProviderTestSuite) TestProcessMachinesConfig() {
	tests := []struct {
		name                string
		machinesConfig      []map[string]interface{}
		orchestratorIP      string
		expectError         bool
		expectedErrorString string
		expectedNodes       int
	}{
		{
			name:                "No orchestrator node and no orchestrator IP",
			machinesConfig:      []map[string]interface{}{},
			expectError:         true,
			expectedErrorString: "no orchestrator node and orchestratorIP is not set",
			expectedNodes:       0,
		},
		{
			name: "No orchestrator node but orchestrator IP specified",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "eastus",
					"parameters": map[string]interface{}{"count": 1},
				},
			},
			orchestratorIP: "1.2.3.4",
			expectError:    false,
			expectedNodes:  1,
		},
		{
			name: "One orchestrator node, no other machines",
			machinesConfig: []map[string]interface{}{
				{"location": "eastus", "parameters": map[string]interface{}{"orchestrator": true}},
			},
			expectError:   false,
			expectedNodes: 1,
		},
		{
			name: "One orchestrator node and many other machines",
			machinesConfig: []map[string]interface{}{
				{"location": "eastus", "parameters": map[string]interface{}{"orchestrator": true}},
				{"location": "westus", "parameters": map[string]interface{}{"count": 3}},
			},
			expectError:   false,
			expectedNodes: 4,
		},
		{
			name: "Multiple orchestrator nodes (should error)",
			machinesConfig: []map[string]interface{}{
				{"location": "eastus", "parameters": map[string]interface{}{"orchestrator": true}},
				{"location": "westus", "parameters": map[string]interface{}{"orchestrator": true}},
			},
			expectError:   true,
			expectedNodes: 0,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest() // Ensure a fresh setup for each sub-test

			deployment, err := models.NewDeployment()
			suite.Require().NoError(err)
			deployment.SSHPrivateKeyPath = suite.testPrivateKeyPath
			deployment.SSHPort = 22
			deployment.OrchestratorIP = tt.orchestratorIP

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}
			suite.azureProvider.SetAzureClient(suite.mockAzureClient)

			viper.Set("azure.machines", tt.machinesConfig)
			viper.Set("general.orchestrator_ip", tt.orchestratorIP)

			machines, locations, err := suite.azureProvider.ProcessMachinesConfig(suite.ctx)

			if tt.expectError {
				suite.Error(err)
				if tt.expectedErrorString != "" {
					suite.Contains(err.Error(), tt.expectedErrorString)
				}
			} else {
				suite.NoError(err)
				suite.Len(machines, tt.expectedNodes)

				if tt.expectedNodes > 0 {
					suite.NotEmpty(locations, "Locations should not be empty")
				}

				if tt.name == "One orchestrator node, no other machines" ||
					tt.name == "One orchestrator node and many other machines" {
					orchestratorFound := false
					for _, machine := range machines {
						if machine.IsOrchestrator() {
							orchestratorFound = true
							break
						}
					}
					suite.True(orchestratorFound)
				} else if tt.name == "No orchestrator node but orchestrator IP specified" {
					suite.NotEmpty(deployment.OrchestratorIP)
					for _, machine := range machines {
						suite.False(machine.IsOrchestrator())
						suite.Equal(deployment.OrchestratorIP, machine.GetOrchestratorIP())
					}
				} else {
					suite.Empty(deployment.OrchestratorIP)
				}
			}
		})
	}
}

func (suite *AzureProviderTestSuite) TestPrepareDeployment() {
	suite.SetupTest() // Ensure a fresh setup for this test

	viper.Set("azure.resource_group_name", "test-rg")
	viper.Set("azure.resource_group_location", "eastus")

	tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
	suite.Require().NoError(err, "Failed to create temporary config file")
	defer os.Remove(tempConfigFile.Name())

	viper.SetConfigFile(tempConfigFile.Name())

	deployment, err := common.PrepareDeployment(suite.ctx, models.DeploymentTypeAzure)
	suite.Require().NoError(err)

	m := display.NewDisplayModel(deployment)
	suite.Require().NotNil(m)

	suite.Contains(deployment.ProjectID, "test-project")
	suite.Contains(deployment.ProjectID, deployment.UniqueID)
	suite.Equal("eastus", deployment.Azure.ResourceGroupLocation)
	suite.NotEmpty(deployment.SSHPublicKeyMaterial)
	suite.NotEmpty(deployment.SSHPrivateKeyMaterial)
	suite.WithinDuration(time.Now(), deployment.StartTime, 20*time.Second)
}

func TestAzureProviderSuite(t *testing.T) {
	suite.Run(t, new(AzureProviderTestSuite))
}
