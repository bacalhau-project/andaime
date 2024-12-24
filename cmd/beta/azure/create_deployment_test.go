package azure

import (
	"context"
	"os"
	"testing"
	"time"

	internal_testutil "github.com/bacalhau-project/andaime/internal/testutil"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	pkg_testutil "github.com/bacalhau-project/andaime/pkg/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CmdBetaAzureCreateDeploymentSuite struct {
	suite.Suite
	ctx                    context.Context
	testSSHPublicKeyPath   string
	testPrivateKeyPath     string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockAzureClient        *azure_mocks.MockAzureClienter
	azureProvider          *azure_provider.AzureProvider
	origGetGlobalModelFunc func() *display.DisplayModel
}

func (suite *CmdBetaAzureCreateDeploymentSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath,
		suite.cleanupPublicKey,
		suite.testPrivateKeyPath,
		suite.cleanupPrivateKey = internal_testutil.CreateSSHPublicPrivateKeyPairOnDisk()

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

func (suite *CmdBetaAzureCreateDeploymentSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
}

func (suite *CmdBetaAzureCreateDeploymentSuite) SetupTest() {
	viper.Reset()
	pkg_testutil.SetupViper(models.DeploymentTypeAzure,
		suite.testSSHPublicKeyPath,
		suite.testPrivateKeyPath,
	)

	var err error
	suite.Require().NoError(err)
}

func (suite *CmdBetaAzureCreateDeploymentSuite) TestProcessMachinesConfig() {
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

			suite.azureProvider, err = azure_provider.NewAzureProviderFunc(
				suite.ctx,
				viper.GetString("azure.subscription_id"),
			)
			suite.Require().NoError(err)
			suite.Require().NotNil(&suite.azureProvider)

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

func (suite *CmdBetaAzureCreateDeploymentSuite) TestPrepareDeployment() {
	suite.SetupTest() // Ensure a fresh setup for this test

	viper.Set("azure.resource_group_name", "test-rg")
	viper.Set("azure.resource_group_location", "eastus")
	viper.Set("general.ssh_public_key_path", suite.testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", suite.testPrivateKeyPath)

	tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
	suite.Require().NoError(err, "Failed to create temporary config file")
	defer os.Remove(tempConfigFile.Name())

	viper.SetConfigFile(tempConfigFile.Name())

	deployment, err := common.PrepareDeployment(suite.ctx, models.DeploymentTypeAzure)
	suite.Require().NoError(err)

	m := display.NewDisplayModel(deployment)
	suite.Require().NotNil(m)

	suite.Contains(deployment.GetProjectID(), "test-project")
	suite.Contains(deployment.UniqueID, deployment.UniqueID)
	suite.Equal("eastus", deployment.Azure.ResourceGroupLocation)
	suite.NotEmpty(deployment.SSHPublicKeyMaterial)
	suite.NotEmpty(deployment.SSHPrivateKeyMaterial)
	suite.WithinDuration(time.Now(), deployment.StartTime, 20*time.Second)

	// Check if machines were properly configured
	suite.Require().Len(deployment.Machines, 1)

	var machine models.Machiner
	for _, m := range deployment.Machines {
		machine = m
		break
	}
	suite.Equal("eastus", machine.GetRegion())
	suite.Equal("Standard_D2s_v3", machine.GetVMSize())
	suite.True(machine.IsOrchestrator())
}

func (suite *CmdBetaAzureCreateDeploymentSuite) TestPrepareDeployment_MissingRequiredFields() {
	testCases := []struct {
		name        string
		setupConfig func()
		expectedErr string
	}{
		{
			name: "Missing resource_group_location",
			setupConfig: func() {
				viper.Set("azure.resource_group_location", "")
			},
			expectedErr: "azure.resource_group_location is not set",
		},
		{
			name: "Missing machines configuration",
			setupConfig: func() {
				viper.Set("azure.machines", nil)
			},
			expectedErr: "no machines configuration found for provider Azure",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // Reset Viper for each test case
			tc.setupConfig()  // Apply the specific configuration for this test case

			// Set a dummy config file to prevent Viper from trying to read a non-existent file
			tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
			suite.Require().NoError(err, "Failed to create temporary config file")
			defer os.Remove(tempConfigFile.Name())
			viper.SetConfigFile(tempConfigFile.Name())

			_, err = common.PrepareDeployment(suite.ctx, models.DeploymentTypeAzure)
			suite.Require().Error(err)
			suite.Contains(err.Error(), tc.expectedErr)
		})
	}
}

func (suite *CmdBetaAzureCreateDeploymentSuite) TestPrepareDeployment_InvalidSSHKeyPaths() {
	viper.Set("general.ssh_public_key_path", "/nonexistent/path/to/public/key")
	viper.Set("general.ssh_private_key_path", "/nonexistent/path/to/private/key")

	_, err := common.PrepareDeployment(suite.ctx, models.DeploymentTypeAzure)
	suite.Require().Error(err)
	suite.Contains(
		err.Error(),
		"failed to extract SSH keys: failed to extract public key material: key file does not exist: /nonexistent/path/to/public/key",
	)
}

func (suite *CmdBetaAzureCreateDeploymentSuite) TestPrepareDeployment_EmptySSHKeyPaths() {
	suite.SetupTest() // Reset Viper for this test case
	viper.Set("general.ssh_public_key_path", "")
	viper.Set("general.ssh_private_key_path", "")

	_, err := common.PrepareDeployment(suite.ctx, models.DeploymentTypeAzure)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to extract SSH keys")
	suite.Contains(err.Error(), "general.ssh_public_key_path is empty")
}

func TestAzureProviderSuite(t *testing.T) {
	suite.Run(t, new(CmdBetaAzureCreateDeploymentSuite))
}
