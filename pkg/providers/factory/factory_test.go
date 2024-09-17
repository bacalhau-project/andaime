// pkg/providers/factory/factory_test.go
package factory

import (
	"context"
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

// TestGetProvider tests the factory's ability to instantiate providers correctly.
func TestGetProvider(t *testing.T) {
	// Initialize Viper for configuration management
	config := viper.New()

	// Define test configurations for Azure and GCP
	testConfigs := map[models.DeploymentType]map[string]interface{}{
		models.DeploymentTypeAzure: {
			"azure.subscription_id":        "test-azure-subscription",
			"azure.resource_group_name":    "test-resource-group",
			"azure.location":               "eastus",
			"general.ssh_public_key_path":  "/path/to/azure/public/key",
			"general.ssh_private_key_path": "/path/to/azure/private/key",
			"general.ssh_user":             "azureuser",
			"general.ssh_port":             22,
		},
		models.DeploymentTypeGCP: {
			"gcp.project_id":               "test-gcp-project",
			"gcp.organization_id":          "test-gcp-org",
			"gcp.billing_account_id":       "test-gcp-billing",
			"gcp.region":                   "us-central1",
			"gcp.zone":                     "us-central1-a",
			"general.ssh_public_key_path":  "/path/to/gcp/public/key",
			"general.ssh_private_key_path": "/path/to/gcp/private/key",
			"general.ssh_user":             "gcpuser",
			"general.ssh_port":             22,
		},
	}

	// Register providers by importing the providers package
	// This ensures that the init functions are executed
	// and providers are registered with the factory
	// The blank import is handled in the providers/init.go file
	// _ = providers.Providers

	// Define test cases
	tests := []struct {
		name           string
		deploymentType models.DeploymentType
		setupConfig    map[string]interface{}
		expectError    bool
	}{
		{
			name:           "Instantiate Azure Provider",
			deploymentType: models.DeploymentTypeAzure,
			setupConfig:    testConfigs[models.DeploymentTypeAzure],
			expectError:    false,
		},
		{
			name:           "Instantiate GCP Provider",
			deploymentType: models.DeploymentTypeGCP,
			setupConfig:    testConfigs[models.DeploymentTypeGCP],
			expectError:    false,
		},
		{
			name:           "Instantiate Unsupported Provider",
			deploymentType: models.DeploymentTypeUnknown,
			setupConfig:    nil, // No configuration needed
			expectError:    true,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// Replace config.Reset() with config = viper.New()
			config = viper.New()

			// If there is a setupConfig, set it in Viper
			if tc.setupConfig != nil {
				for key, value := range tc.setupConfig {
					config.Set(key, value)
				}
			}

			// GetProvider should return a Providerer instance or an error
			provider, err := GetProvider(context.Background(), tc.deploymentType)

			if tc.expectError {
				if err == nil {
					t.Fatalf(
						"Expected error when instantiating provider type %s, but got none",
						tc.deploymentType,
					)
				} else {
					t.Logf("Received expected error: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to instantiate provider type %s: %v", tc.deploymentType, err)
			}

			if provider == nil {
				t.Fatalf("Provider instance is nil for provider type %s", tc.deploymentType)
			}

			// Optional: Check if the provider is of the expected type
			/*
				switch tc.deploymentType {
				case models.DeploymentTypeAzure:
					if _, ok := provider.(*azure.AzureProvider); !ok {
						t.Errorf("Expected AzureProvider type, but got %T", provider)
					}
				case models.DeploymentTypeGCP:
					if _, ok := provider.(*gcp.GCPProvider); !ok {
						t.Errorf("Expected GCPProvider type, but got %T", provider)
					}
				}
			*/

			t.Logf("Provider type %s instantiated successfully: %+v", tc.deploymentType, provider)
		})
	}
}

// Example function to ensure the providers are imported and registered.
// This is redundant if the providers are already imported via the providers/init.go
func TestProvidersRegistration(t *testing.T) {
	// Ensure that the factory registry contains the expected providers
	expectedProviders := []models.DeploymentType{
		models.DeploymentTypeAzure,
		models.DeploymentTypeGCP,
	}

	for _, pt := range expectedProviders {
		if _, exists := registry[pt]; !exists {
			t.Errorf("Provider type %s is not registered in the factory", pt)
		}
	}
}

func TestProviders(t *testing.T) {
	tests := []struct {
		name           string
		deploymentType models.DeploymentType
		configSetup    func(config *viper.Viper)
		expectError    bool
	}{
		{
			name:           "Azure Provider with valid config",
			deploymentType: models.DeploymentTypeAzure,
			configSetup: func(config *viper.Viper) {
				config.Set("azure.subscription_id", "valid-subscription-id")
				config.Set("azure.resource_group_name", "test-resource-group")
				config.Set("azure.location", "eastus")
				config.Set("general.ssh_public_key_path", "/path/to/azure/public/key")
				config.Set("general.ssh_private_key_path", "/path/to/azure/private/key")
				config.Set("general.ssh_user", "azureuser")
				config.Set("general.ssh_port", 22)
			},
			expectError: false,
		},
		// Add more test cases as needed
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			config := viper.New()
			tc.configSetup(config)

			// Update the GetProvider call to match its signature
			provider, err := GetProvider(context.Background(), tc.deploymentType)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for provider type %s, but got none", tc.deploymentType)
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect error for provider type %s, but got: %v", tc.deploymentType, err)
				}
				if provider == nil {
					t.Errorf("Provider instance is nil for provider type %s", tc.deploymentType)
				}
			}
		})
	}
}

// To visualize the factory registry, you can add a helper function
func ExamplePrintRegisteredProviders() {
	fmt.Println("Registered Providers:")
	for k := range registry {
		fmt.Println(k)
	}
	// Output:
	// Registered Providers:
	// AZURE
	// GCP
}
