package azure

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitializeDeployment(t *testing.T) {
	// Set up the base path for test data
	testDataPath := os.Getenv("TEST_DATA_PATH")
	if testDataPath == "" {
		testDataPath = "../../.." // Adjust this based on your project structure
	}

	tempConfigFile, err := os.CreateTemp("", "azure_test_config_*.yaml")
	assert.NoError(t, err, "Failed to create temporary config file")
	defer os.Remove(tempConfigFile.Name()) // Clean up the temporary file after the test

	configContent, err := testdata.ReadTestAzureConfig()
	assert.NoError(t, err, "Failed to read test config")
	_, err = tempConfigFile.Write([]byte(configContent))
	assert.NoError(t, err, "Failed to write to temporary config file")
	tempConfigFile.Close()

	// Set up Viper to read from the temporary config file
	viper.SetConfigFile(tempConfigFile.Name())
	err = viper.ReadInConfig()
	// Add
	assert.NoError(t, err, "Failed to read config file")
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

	// Call InitializeDeployment
	ctx := context.Background()
	uniqueID := "test123"
	deployment, err := InitializeDeployment(ctx, uniqueID)
	assert.NoError(t, err, "InitializeDeployment failed")

	// Check the total number of machines
	assert.Equal(t, 9, len(deployment.Machines), "Expected 9 machines in total")

	// Check the number of unique locations
	locations := make(map[string]bool)
	for _, machine := range deployment.Machines {
		locations[machine.Location] = true
	}
	assert.Equal(t, 5, len(locations), "Expected 5 unique locations")

	// Check specific configurations for each location
	eastus2Count := 0
	westusCount := 0
	brazilsouthCount := 0
	euwestCount := 0
	asiawestCount := 0

	for _, machine := range deployment.Machines {
		switch machine.Location {
		case "eastus2":
			eastus2Count++
			assert.Equal(
				t,
				"Standard_DS1_v4",
				machine.VMSize,
				"Expected eastus2 machines to be Standard_DS1_v4",
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
		case "euwest":
			euwestCount++
			assert.True(
				t,
				machine.Orchestrator,
				"Expected euwest machine to be orchestrator",
			)
		case "asiawest":
			asiawestCount++
		}
	}

	// Verify the count of machines in each location
	assert.Equal(t, 2, eastus2Count, "Expected 2 machines in eastus2")
	assert.Equal(t, 4, westusCount, "Expected 4 machines in westus")
	assert.Equal(t, 1, brazilsouthCount, "Expected 1 machine in brazilsouth")
	assert.Equal(t, 1, euwestCount, "Expected 1 machine in euwest")
	assert.Equal(t, 1, asiawestCount, "Expected 1 machine in asiawest")

	// Verify that only one orchestrator exists
	orchestratorCount := 0
	for _, machine := range deployment.Machines {
		if machine.Orchestrator {
			orchestratorCount++
		}
	}
	assert.Equal(t, 1, orchestratorCount, "Expected exactly one orchestrator machine")
}

func TestProcessMachinesConfig(t *testing.T) {
	tempPrivateKey, err := os.CreateTemp("", "dummy_private_key")
	assert.NoError(t, err, "Failed to create temporary private key file")
	_, err = tempPrivateKey.Write([]byte(testdata.TestPrivateSSHKeyMaterial))
	assert.NoError(t, err, "Failed to write to temporary private key file")
	tempPrivateKey.Close()
	defer os.Remove(tempPrivateKey.Name())

	tempPublicKey, err := os.CreateTemp("", "dummy_public_key")
	assert.NoError(t, err, "Failed to create temporary public key file")
	_, err = tempPublicKey.Write([]byte(testdata.TestPublicSSHKeyMaterial))
	assert.NoError(t, err, "Failed to write to temporary public key file")
	tempPublicKey.Close()
	defer os.Remove(tempPublicKey.Name())

	deployment := &models.Deployment{
		SSHPrivateKeyPath: tempPrivateKey.Name(),
		SSHPort:           22,
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
			deployment.Machines = map[string]*models.Machine{}
			deployment.OrchestratorIP = ""
			if tt.orchestratorIP != "" {
				deployment.OrchestratorIP = tt.orchestratorIP
			}

			// Reset viper config for each test
			viper.Reset()
			viper.Set("azure.machines", tt.machinesConfig)
			viper.Set("azure.default_count_per_zone", 1)
			viper.Set("azure.default_machine_type", "Standard_D2s_v3")
			viper.Set("azure.disk_size_gb", 30)
			viper.Set("general.orchestrator_ip", tt.orchestratorIP)

			err := ProcessMachinesConfig(deployment)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, deployment.Machines, tt.expectedNodes)

				if tt.expectedNodes > 0 {
					assert.NotEmpty(t, deployment.UniqueLocations)
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
