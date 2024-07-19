package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/bacalhau-project/andaime/utils"
	"github.com/spf13/viper"
)

func TestConfigFileReading(t *testing.T) {
	// Test successful config file reading
	t.Run("SuccessfulReading", func(t *testing.T) {
		v := viper.New()
		v.SetConfigFile("config.yml")
		err := v.ReadInConfig()
		if err != nil {
			t.Fatalf("Failed to read config file: %v", err)
		}

		// Check if some expected keys are present
		expectedKeys := []string{"general.project_id", "azure.subscription_id"}
		for _, key := range expectedKeys {
			if !v.IsSet(key) {
				t.Errorf("Expected key %s not found in config", key)
			}
		}
	})

	// Test handling of missing required fields
	t.Run("MissingRequiredFields", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "test_config_*.yml")
		defer os.Remove(tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Write an incomplete config to the temp file
		incompleteConfig := []byte(`
general:
  project_id: "test-project"
azure:
  subscription_id: "test-subscription"
`)
		if _, err := tempFile.Write(incompleteConfig); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		v := viper.New()
		v.SetConfigFile(tempFile.Name())
		err = v.ReadInConfig()
		if err != nil {
			t.Fatalf("Failed to read config file: %v", err)
		}

		// Check for missing required fields
		requiredFields := []string{"azure.resource_group", "azure.vm_name", "azure.vm_size"}
		missingFields := []string{}
		for _, field := range requiredFields {
			if !v.IsSet(field) {
				missingFields = append(missingFields, field)
			}
		}

		if len(missingFields) == 0 {
			t.Error("Expected missing required fields, but found none")
		}

		// Here you would typically call a function that checks for required fields
		// and returns a clear error message. For example:
		// err = validateConfig(v)
		// if err == nil {
		//     t.Error("Expected error for missing required fields, but got nil")
		// }
		// if !strings.Contains(err.Error(), "Missing required fields: resource_group, vm_name, vm_size") {
		//     t.Errorf("Error message doesn't contain expected content. Got: %v", err)
		// }
	})

	// Test handling of invalid values
	t.Run("InvalidValues", func(t *testing.T) {
		tempFile, err := ioutil.TempFile("", "test_config_*.yml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Write a config with invalid values to the temp file
		invalidConfig := []byte(`
general:
  project_id: "test-project"
azure:
  subscription_id: "test-subscription"
  resource_group: "test-group"
  vm_name: "test-vm"
  vm_size: "Invalid_Size"
  disk_size_gb: -10
`)
		if _, err := tempFile.Write(invalidConfig); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		v := viper.New()
		v.SetConfigFile(tempFile.Name())
		err = v.ReadInConfig()
		if err != nil {
			t.Fatalf("Failed to read config file: %v", err)
		}

		// Here you would typically call a function that validates the values
		// and returns a clear error message. For example:
		// err = validateConfigValues(v)
		// if err == nil {
		//     t.Error("Expected error for invalid values, but got nil")
		// }
		// expectedErrors := []string{"Invalid VM size", "Disk size must be positive"}
		// for _, expectedErr := range expectedErrors {
		//     if !strings.Contains(err.Error(), expectedErr) {
		//         t.Errorf("Error message doesn't contain expected content. Got: %v", err)
		//     }
		// }
	})
	// Test minimal valid configuration
	t.Run("MinimalValidConfig", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "test_config_*.yml")
		defer os.Remove(tempFile.Name())

		tempPrivateKey, err := os.CreateTemp("", "id_rsa")
		tempPrivateKey.Write([]byte(utils.TestPrivateSSHKey))
		defer os.Remove(tempPrivateKey.Name())

		tempPublicKey, err := os.CreateTemp("", "id_rsa.pub")
		tempPublicKey.Write([]byte(utils.TestPublicSSHKey))
		defer os.Remove(tempPublicKey.Name())

		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Write a minimal valid config to the temp file
		minimalConfig := []byte(fmt.Sprintf(`
general:
  project_id: "test-project"
  ssh_public_key_path: "%s"
azure:
  subscription_id: "test-subscription"
  resource_group: "test-group"
  locations:
    - name: "eastus"
      zones:
        - name: "1"
          machines:
            - type: "Standard_DS1_v2"
              count: 1
  allowed_ports:
    - 22
    - 80
    - 443
  disk_size_gb: 30
`, tempPublicKey.Name()))
		if _, err := tempFile.Write(minimalConfig); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		v := viper.New()
		v.SetConfigFile(tempFile.Name())
		err = v.ReadInConfig()
		if err != nil {
			t.Fatalf("Failed to read config file: %v", err)
		}

		err = validateConfig(v)
		if err != nil {
			t.Errorf("Config validation failed for minimal valid config: %v", err)
		}

		err = validateConfigValues(v)
		if err != nil {
			t.Errorf("Config value validation failed for minimal valid config: %v", err)
		}
	})

	// Test invalid location structure
	t.Run("InvalidLocationStructure", func(t *testing.T) {
		tempFile, err := ioutil.TempFile("", "test_config_*.yml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		invalidConfig := []byte(`
general:
  project_id: "test-project"
  ssh_public_key_path: "~/.ssh/id_rsa.pub"
azure:
  subscription_id: "test-subscription"
  resource_group: "test-group"
  locations:
    - name: "eastus"
      # Missing zones
  allowed_ports:
    - 22
    - 80
    - 443
  disk_size_gb: 30
`)
		if _, err := tempFile.Write(invalidConfig); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		v := viper.New()
		v.SetConfigFile(tempFile.Name())
		err = v.ReadInConfig()
		if err != nil {
			t.Fatalf("Failed to read config file: %v", err)
		}

		err = validateConfigValues(v)
		if err == nil {
			t.Error("Expected error for invalid location structure, but got nil")
		} else if !strings.Contains(err.Error(), "location 0 is missing 'zones'") {
			t.Errorf("Error message doesn't contain expected content. Got: %v", err)
		}
	})
}