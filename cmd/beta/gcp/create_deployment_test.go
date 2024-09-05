package gcp

import (
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCreateDeploymentCmd(t *testing.T) {
	cmd := GetGCPCreateDeploymentCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "create-deployment", cmd.Use)
	assert.Equal(t, "Create a deployment in GCP", cmd.Short)
}

func TestProcessMachinesConfig(t *testing.T) {
	// Set up the test configuration
	viper.Set("gcp.default_count_per_zone", 1)
	viper.Set("gcp.default_machine_type", "n1-standard-1")
	viper.Set("gcp.disk_size_gb", 10)
	viper.Set("gcp.organization_id", "test-org-id")
	viper.Set("general.project_prefix", "andaime-test")
	viper.Set("general.ssh_private_key", "test-ssh-private-key")
	viper.Set("general.ssh_port", 22)
	_,
		cleanupPublicKey,
		testPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	deployment, err := models.NewDeployment()
	assert.NoError(t, err)
	deployment.SSHPrivateKeyPath = testPrivateKeyPath
	deployment.SSHPort = 22

	// Test case 1: Valid configuration
	t.Run("Valid configuration", func(t *testing.T) {
		viper.Set("gcp.machines", []map[string]interface{}{
			{
				"location": "us-central1-a",
				"parameters": map[string]interface{}{
					"count":        1,
					"type":         "n2-standard-2",
					"orchestrator": true,
				},
			},
		})

		err = ProcessMachinesConfig(deployment)
		assert.NoError(t, err)
		assert.Len(t, deployment.Machines, 1)
	})

	// Test case 2: Invalid location
	t.Run("Invalid location", func(t *testing.T) {
		viper.Set("gcp.machines", []map[string]interface{}{
			{
				"location": "bad-location-2", // Invalid GCP location
				"parameters": map[string]interface{}{
					"count": 1,
					"type":  "n1-standard-1",
				},
			},
		})

		deployment, err := models.NewDeployment()
		deployment.SSHPrivateKeyPath = testPrivateKeyPath
		deployment.SSHPort = 22

		assert.NoError(t, err)

		err = ProcessMachinesConfig(deployment)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid location for GCP: bad-location-2")
	})
}
