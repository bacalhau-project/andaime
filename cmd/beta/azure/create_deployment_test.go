package azure

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/internal/testutil"
	andaime_mocks "github.com/bacalhau-project/andaime/mocks"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTest() {
	_ = logger.Get()
}

func TestProcessMachinesConfig(t *testing.T) {
	ctx := context.Background()
	testSSHPublicKeyPath,
		cleanupPublicKey,
		testPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockAzureClient := new(andaime_mocks.MockClienter)
	mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	viper.Set("general.ssh_private_key_path", testPrivateKeyPath)
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
	viper.Set("azure.subscription_id", "test-subscription-id")
	// Create a mock AzureProvider
	p, err := azure_provider.NewAzureProvider(ctx, "test-subscription-id")
	assert.NoError(t, err)
	assert.NotNil(t, p)
	azureProvider := p.(azure.AzureProviderer)

	deployment, err := models.NewDeployment()
	assert.NoError(t, err)
	deployment.SSHPrivateKeyPath = testPrivateKeyPath
	deployment.SSHPort = 22

	origGetGlobalModelFunc := display.GetGlobalModelFunc
	t.Cleanup(func() { display.GetGlobalModelFunc = origGetGlobalModelFunc })
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

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
		t.Run(tt.name, func(t *testing.T) {
			// Reset the machines slice for each test
			deployment.Machines = make(map[string]models.Machiner)
			deployment.OrchestratorIP = ""
			if tt.orchestratorIP != "" {
				deployment.OrchestratorIP = tt.orchestratorIP
			}

			// Reset viper config for each test
			viper.Reset()
			viper.Set("azure.machines", tt.machinesConfig)
			viper.Set("azure.default_count_per_zone", 1)
			viper.Set("azure.default_machine_type", "Standard_DS4_v2")
			viper.Set("azure.default_disk_size_gb", 30)
			viper.Set("general.orchestrator_ip", tt.orchestratorIP)

			// Use the mock AzureProvider instead of creating a new one
			machines, locations, err := azureProvider.ProcessMachinesConfig(ctx)
			deployment.SetMachines(machines)
			deployment.SetLocations(locations)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, deployment.Machines, tt.expectedNodes)

				if tt.expectedNodes > 0 {
					assert.NotEmpty(t, deployment.Locations, "Locations should not be empty")
				}

				// Check if orchestrator node is set when expected
				//
				if tt.name == "One orchestrator node, no other machines" ||
					tt.name == "One orchestrator node and many other machines" {
					// Go through all the machines in the deployment and check if the orchestrator bit is set on one (and only one) of them
					orchestratorFound := false
					for _, machine := range deployment.GetMachines() {
						if machine.IsOrchestrator() {
							orchestratorFound = true
						}
					}
					assert.True(t, orchestratorFound)
				} else if tt.name == "No orchestrator node but orchestrator IP specified" {
					assert.NotNil(t, deployment.OrchestratorIP)
					for _, machine := range deployment.Machines {
						assert.False(t, machine.IsOrchestrator())
						assert.Equal(t, deployment.OrchestratorIP, machine.GetOrchestratorIP())
					}
				} else {
					assert.Nil(t, deployment.OrchestratorIP)
				}
			}
		})
	}
}

func TestPrepareDeployment(t *testing.T) {
	testSSHPublicKeyPath,
		cleanupPublicKey,
		testPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockAzureClient := new(azure.MockAzureClient)
	mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)
	// Setup
	ctx := context.Background()

	// Mock viper configuration
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", testPrivateKeyPath)
	viper.Set("azure.subscription_id", "test-subscription-id")
	viper.Set("azure.resource_group_name", "test-rg")
	viper.Set("azure.resource_group_location", "eastus")
	viper.Set("azure.default_count_per_zone", 1)
	viper.Set("azure.default_machine_type", "Standard_DS4_v2")
	viper.Set("azure.default_disk_size_gb", int32(30))
	tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
	assert.NoError(t, err, "Failed to create temporary config file")
	defer os.Remove(tempConfigFile.Name())

	viper.SetConfigFile(tempConfigFile.Name())

	// Execute
	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	deployment, err := common.PrepareDeployment(ctx, models.DeploymentTypeAzure)
	assert.NoError(t, err)

	m := display.NewDisplayModel(deployment)
	assert.NotNil(t, m)

	// Assert
	assert.Contains(t, deployment.ProjectID, "test-project")
	assert.Contains(t, deployment.ProjectID, deployment.UniqueID)
	assert.Equal(t, "eastus", deployment.Azure.ResourceGroupLocation)
	assert.NotEmpty(t, deployment.SSHPublicKeyMaterial)
	assert.NotEmpty(t, deployment.SSHPrivateKeyMaterial)
	assert.WithinDuration(t, time.Now(), deployment.StartTime, 20*time.Second)
}
