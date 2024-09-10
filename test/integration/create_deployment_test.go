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

func setupViper(
	t *testing.T,
	provider models.DeploymentType,
	testSSHPublicKeyPath, testSSHPrivateKeyPath string,
) {
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

			// Create a mock provider
			mockProvider := &common.MockProvider{}
			mockProvider.On("CreateDeployment", mock.Anything, mock.Anything).Return(nil)

			// Inject the mock provider
			var err error
			if tt.provider == models.DeploymentTypeAzure {
				err = azure.ExecuteCreateDeployment(cmd, []string{}, mockProvider)
			} else {
				err = gcp.ExecuteCreateDeployment(cmd, []string{}, mockProvider)
			}

			assert.NoError(t, err)

			// Assert that CreateDeployment was called
			mockProvider.AssertCalled(t, "CreateDeployment", mock.Anything, mock.Anything)

			// Check if the deployment was created with the correct configuration
			deployment := mockProvider.Calls[0].Arguments.Get(1).(*models.Deployment)
			assert.NotEmpty(t, deployment.Name)
			assert.Equal(t, tt.provider, deployment.Type)
			assert.NotEmpty(t, deployment.Machines)

			// Check if SSH keys were properly set
			assert.Equal(t, testSSHPublicKeyPath, deployment.SSHPublicKeyPath)
			assert.Equal(t, testSSHPrivateKeyPath, deployment.SSHPrivateKeyPath)

			// Check provider-specific configurations
			if tt.provider == models.DeploymentTypeAzure {
				assert.Equal(t, "4a45a76b-5754-461d-84a1-f5e47b0a7198", deployment.AzureConfig.SubscriptionID)
				assert.Equal(t, "test-1292-rg", deployment.AzureConfig.ResourceGroupName)
			} else {
				assert.Equal(t, "test-1292-gcp", deployment.GCPConfig.ProjectID)
				assert.Equal(t, "org-1234567890", deployment.GCPConfig.OrganizationID)
			}
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
			assert.Equal(t, tt.provider, deployment.Type)
			assert.NotEmpty(t, deployment.Machines)

			// Check if SSH keys were properly set
			assert.Equal(t, testSSHPublicKeyPath, deployment.SSHPublicKeyPath)
			assert.Equal(t, testSSHPrivateKeyPath, deployment.SSHPrivateKeyPath)

			// Check if the correct number of machines were created
			assert.Len(t, deployment.Machines, 3) // 2 regular + 1 orchestrator

			// Check if there's exactly one orchestrator
			orchestrators := 0
			for _, machine := range deployment.Machines {
				if machine.Parameters["orchestrator"] == true {
					orchestrators++
				}
			}
			assert.Equal(t, 1, orchestrators)

			// Check provider-specific configurations
			if tt.provider == models.DeploymentTypeAzure {
				assert.Equal(t, "4a45a76b-5754-461d-84a1-f5e47b0a7198", deployment.AzureConfig.SubscriptionID)
				assert.Equal(t, "test-1292-rg", deployment.AzureConfig.ResourceGroupName)
				assert.Equal(t, "eastus2", deployment.AzureConfig.ResourceGroupLocation)
				assert.Equal(t, 30, deployment.AzureConfig.DefaultDiskSizeGB)
			} else {
				assert.Equal(t, "test-1292-gcp", deployment.GCPConfig.ProjectID)
				assert.Equal(t, "org-1234567890", deployment.GCPConfig.OrganizationID)
				assert.Equal(t, "123456-789012-345678", deployment.GCPConfig.BillingAccountID)
				assert.Equal(t, 30, deployment.GCPConfig.DefaultDiskSizeGB)
			}

			// Check machine configurations
			for _, machine := range deployment.Machines {
				assert.NotEmpty(t, machine.Name)
				assert.NotEmpty(t, machine.Location)
				if tt.provider == models.DeploymentTypeAzure {
					assert.Contains(t, []string{"eastus2", "westus"}, machine.Location)
					assert.Equal(t, "Standard_DS4_v2", machine.Parameters["machine_type"])
				} else {
					assert.Contains(t, []string{"us-central1-a", "us-central1-b"}, machine.Location)
					assert.Equal(t, "n2-highcpu-4", machine.Parameters["machine_type"])
				}
			}
		})
	}
}
