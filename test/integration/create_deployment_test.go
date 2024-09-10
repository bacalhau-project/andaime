package integration

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/bacalhau-project/andaime/cmd"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

func setupTestEnvironment(t *testing.T) (func(), string, string) {
	os.Setenv("ANDAIME_TEST_MODE", "true")
	testSSHPublicKeyPath, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	cleanup := func() {
		os.Unsetenv("ANDAIME_TEST_MODE")
		cleanupPublicKey()
		cleanupPrivateKey()
	}

	return cleanup, testSSHPublicKeyPath, testSSHPrivateKeyPath
}

func setupViper(t *testing.T, provider models.DeploymentType, testSSHPublicKeyPath, testSSHPrivateKeyPath string) {
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
		viper.Set("azure.machines", []map[string]interface{}{
			{
				"location": "eastus2",
				"parameters": map[string]interface{}{
					"count": 2,
				},
			},
			{
				"location": "westus",
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
		viper.Set("gcp.default_machine_type", "n2-highcpu-4")
		viper.Set("gcp.default_disk_size_gb", 30)
		viper.Set("gcp.machines", []map[string]interface{}{
			{
				"location": "us-central1-a",
				"parameters": map[string]interface{}{
					"count": 2,
				},
			},
			{
				"location": "us-central1-b",
				"parameters": map[string]interface{}{
					"orchestrator": true,
				},
			},
		})
	}
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

			// TODO: Implement provider-specific mocks and assertions
			// This is where you'd set up your mocks for Azure or GCP clients

			var err error
			if tt.provider == models.DeploymentTypeAzure {
				// err = azure.ExecuteCreateDeployment(cmd, []string{})
			} else {
				// err = gcp.ExecuteCreateDeployment(cmd, []string{})
			}

			assert.NoError(t, err)

			// TODO: Add more specific assertions based on the expected behavior
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
			assert.NoError(t, err)
			assert.NotNil(t, deployment)

			assert.NotEmpty(t, deployment.Name)
			assert.NotEmpty(t, deployment.Machines)

			// TODO: Add more specific assertions based on the expected deployment configuration
		})
	}
}
