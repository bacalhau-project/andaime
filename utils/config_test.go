package utils

import (
	"os"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	tempFile, err := os.CreateTemp("", "config.*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write test configuration to the temp file
	testConfig := []byte(`
aws:
  regions:
    - us-west-2
    - us-east-1
    - eu-west-1
`)
	if _, err := tempFile.Write(testConfig); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Test LoadConfig function
	config, err := LoadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check if the loaded config matches the expected values
	expectedRegions := []string{"us-west-2", "us-east-1", "eu-west-1"}
	if !reflect.DeepEqual(config.AWS.Regions, expectedRegions) {
		t.Errorf("Loaded regions do not match expected. Got %v, want %v", config.AWS.Regions, expectedRegions)
	}
}

func TestLoadConfigError(t *testing.T) {
	// Test with non-existent file
	_, err := LoadConfig("non_existent_file.yaml")
	if err == nil {
		t.Error("Expected an error when loading non-existent file, but got nil")
	}

	// Test with invalid YAML content
	tempFile, err := os.CreateTemp("", "invalid_config.*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	invalidConfig := []byte(`
aws:
  regions:
    - us-west-2
    - us-east-1
  : invalid-yaml
`)
	if _, err := tempFile.Write(invalidConfig); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	_, err = LoadConfig(tempFile.Name())
	if err == nil {
		t.Error("Expected an error when loading invalid YAML, but got nil")
	}
}