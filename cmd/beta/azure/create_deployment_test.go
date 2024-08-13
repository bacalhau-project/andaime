package azure

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitializeDeployment(t *testing.T) {
	disp := display.GetGlobalDisplay()
	disp.Visible = false

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
				machine.Parameters.Orchestrator,
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
		if machine.Parameters.Orchestrator {
			orchestratorCount++
		}
	}
	assert.Equal(t, 1, orchestratorCount, "Expected exactly one orchestrator machine")
}
