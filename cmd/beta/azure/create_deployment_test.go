package azure

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessMachinesConfig(t *testing.T) {
	_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockAzureClient := new(azure.MockAzureClient)
	mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	origNewAzureClientFunc := azure.NewAzureClientFunc
	t.Cleanup(func() { azure.NewAzureClientFunc = origNewAzureClientFunc })
	azure.NewAzureClientFunc = func(subscriptionID string) (azure.AzureClienter, error) {
		return mockAzureClient, nil
	}

	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	deployment, err := models.NewDeployment(models.DeploymentTypeAzure)
	assert.NoError(t, err)
	deployment.SSHPrivateKeyPath = testPrivateKeyPath
	deployment.SSHPort = 22

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
			deployment.Machines = map[string]*models.Machine{}
			deployment.OrchestratorIP = ""
			if tt.orchestratorIP != "" {
				deployment.OrchestratorIP = tt.orchestratorIP
			}

			// Reset viper config for each test
			viper.Reset()
			viper.Set("azure.machines", tt.machinesConfig)
			viper.Set("azure.default_count_per_zone", 1)
			viper.Set("azure.default_machine_type", "Standard_DS4_v2")
			viper.Set("azure.disk_size_gb", 30)
			viper.Set("general.orchestrator_ip", tt.orchestratorIP)

			err := ProcessMachinesConfig(deployment)

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
				if tt.name == "One orchestrator node, no other machines" || tt.name == "One orchestrator node and many other machines" {
					// Go through all the machines in the deployment and check if the orchestrator bit is set on one (and only one) of them
					orchestratorFound := false
					for _, machine := range deployment.Machines {
						if machine.Orchestrator {
							orchestratorFound = true
						}
					}
					assert.True(t, orchestratorFound)
				} else if tt.name == "No orchestrator node but orchestrator IP specified" {
					assert.NotNil(t, deployment.OrchestratorIP)
					for _, machine := range deployment.Machines {
						assert.False(t, machine.Orchestrator)
						assert.Equal(t, deployment.OrchestratorIP, machine.OrchestratorIP)
					}
				} else {
					assert.Nil(t, deployment.OrchestratorIP)
				}
			}
		})
	}
}
func TestInitializeDeployment(t *testing.T) {
	viper.Reset()

	// Save original environment
	originalEnv := os.Environ()
	t.Cleanup(func() {
		os.Clearenv()
		for _, pair := range originalEnv {
			parts := strings.SplitN(pair, "=", 2)
			os.Setenv(parts[0], parts[1])
		}
	})
	tempDir, err := os.MkdirTemp("", "test_initialize_deployment")
	assert.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// Set up the base path for test data
	testDataPath := os.Getenv("TEST_DATA_PATH")
	if testDataPath == "" {
		testDataPath = "../../.." // Adjust this based on your project structure
	}

	// Create and populate temporary config file
	tempConfigFile, err := os.CreateTemp(tempDir, "azure_test_config_*.yaml")
	assert.NoError(t, err)
	configContent, err := testdata.ReadTestAzureConfig()
	assert.NoError(t, err)
	_, err = tempConfigFile.Write([]byte(configContent))
	assert.NoError(t, err)
	tempConfigFile.Close()

	// Set up Viper to read from the temporary config file
	viper.SetConfigFile(tempConfigFile.Name())
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	// Update the SSH key paths in the configuration
	viper.Set(
		"general.ssh_public_key_path",
		filepath.Join(testDataPath, "internal", "testdata", "dummy_keys", "id_ed25519.pub"),
	)
	viper.Set(
		"general.ssh_private_key_path",
		filepath.Join(testDataPath, "internal", "testdata", "dummy_keys", "id_ed25519"),
	)
	viper.Set("general.ssh_key_dir", filepath.Join(testDataPath, "testdata", "dummy_keys"))

	// Create a local display model for this test
	deployment, err := models.NewDeployment(models.DeploymentTypeAzure)
	assert.NoError(t, err)
	localModel := display.InitialModel(deployment)
	origGetGlobalModel := display.GetGlobalModelFunc
	t.Cleanup(func() { display.GetGlobalModelFunc = origGetGlobalModel })
	display.GetGlobalModelFunc = func() *display.DisplayModel { return localModel }

	mockAzureClient := new(azure.MockAzureClient)
	mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	origNewAzureClientFunc := azure.NewAzureClientFunc
	t.Cleanup(func() { azure.NewAzureClientFunc = origNewAzureClientFunc })
	azure.NewAzureClientFunc = func(subscriptionID string) (azure.AzureClienter, error) {
		return mockAzureClient, nil
	}

	// Run subtests
	t.Run("PrepareDeployment", func(t *testing.T) {
		ctx := context.Background()
		viper.Set("general.project_prefix", "test-project")
		viper.Set("general.unique_id", "test-unique-id")
		deployment, err := PrepareDeployment(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, deployment)
		localModel.Deployment = deployment

		// Check the total number of machines
		assert.Equal(t, 9, len(localModel.Deployment.Machines), "Expected 9 machines in total")

		// Check the number of unique locations
		locations := make(map[string]bool)
		for _, machine := range localModel.Deployment.Machines {
			locations[machine.Location] = true
		}
		assert.Equal(t, 5, len(locations), "Expected 5 unique locations")

		// Check specific configurations for each location
		eastus2Count := 0
		westusCount := 0
		brazilsouthCount := 0
		ukwestCount := 0
		uaenorthCount := 0

		for _, machine := range localModel.Deployment.Machines {
			switch machine.Location {
			case "eastus2":
				eastus2Count++
				assert.Equal(
					t,
					"Standard_DS1_v4",
					machine.VMSize,
					"Expected eastus machines to be Standard_DS1_v4",
				)
			case "westus":
				westusCount++
			case "brazilsouth":
				brazilsouthCount++
				assert.Equal(
					t,
					"Standard_DS1_v8",
					machine.VMSize,
					"Expected brazilsouth machines to be Standard_DS1_v8",
				)
			case "ukwest":
				ukwestCount++
				assert.True(t, machine.Orchestrator, "Expected ukwest machine to be orchestrator")
			case "uaenorth":
				uaenorthCount++
			}
		}

		// Verify the count of machines in each location
		assert.Equal(t, 2, eastus2Count, "Expected 2 machines in eastus2")
		assert.Equal(t, 4, westusCount, "Expected 4 machines in westus")
		assert.Equal(t, 1, brazilsouthCount, "Expected 1 machine in brazilsouth")
		assert.Equal(t, 1, ukwestCount, "Expected 1 machine in ukwest")
		assert.Equal(t, 1, uaenorthCount, "Expected 1 machine in uaenorth")

		// Verify that only one orchestrator exists
		orchestratorCount := 0
		for _, machine := range localModel.Deployment.Machines {
			if machine.Orchestrator {
				orchestratorCount++
			}
		}
		assert.Equal(t, 1, orchestratorCount, "Expected exactly one orchestrator machine")
	})
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

	origNewAzureClientFunc := azure.NewAzureClientFunc
	t.Cleanup(func() { azure.NewAzureClientFunc = origNewAzureClientFunc })
	azure.NewAzureClientFunc = func(subscriptionID string) (azure.AzureClienter, error) {
		return mockAzureClient, nil
	}
	t.Cleanup(func() { azure.NewAzureClientFunc = origNewAzureClientFunc })

	// Setup
	ctx := context.Background()

	// Mock viper configuration
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", testPrivateKeyPath)
	viper.Set("azure.subscription_id", "test-subscription-id")
	viper.Set("azure.resource_group_name", "test-rg")
	viper.Set("azure.resource_group_location", "")
	viper.Set("azure.default_count_per_zone", 1)
	viper.Set("azure.default_machine_type", "Standard_DS4_v2")
	viper.Set("azure.disk_size_gb", int32(30))
	viper.Set("azure.machines", []map[string]interface{}{
		{
			"location": "eastus",
			"parameters": map[string]interface{}{
				"orchestrator": true,
			},
		},
	})
	tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
	assert.NoError(t, err, "Failed to create temporary config file")
	defer os.Remove(tempConfigFile.Name())

	viper.SetConfigFile(tempConfigFile.Name())

	// Execute
	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	deployment, err := PrepareDeployment(ctx)
	assert.NotNil(t, deployment)
	assert.NoError(t, err)

	display.SetGlobalModel(display.InitialModel(deployment))
	// Assert
	assert.NoError(t, err)
	assert.Contains(t, deployment.ProjectID, "test-project")
	assert.Contains(t, deployment.ProjectID, deployment.UniqueID)
	assert.Equal(t, "eastus", deployment.ResourceGroupLocation)
	assert.NotEmpty(t, deployment.SSHPublicKeyMaterial)
	assert.NotEmpty(t, deployment.SSHPrivateKeyMaterial)
	assert.WithinDuration(t, time.Now(), deployment.StartTime, 20*time.Second)
	assert.Len(t, deployment.Machines, 1)

	var machine *models.Machine
	for _, m := range deployment.Machines {
		machine = m
		break
	}
	assert.Equal(t, "eastus", machine.Location)
	assert.True(t, machine.Orchestrator)
}
