package gcp

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestProcessMachinesConfig(t *testing.T) {
	// Set up the test configuration
	viper.Set("gcp.default_count_per_zone", 1)
	viper.Set("gcp.default_machine_type", "n1-standard-1")
	viper.Set("gcp.disk_size_gb", 10)
	viper.Set("gcp.organization_id", "test-org-id")

	// Test case 1: Valid configuration
	t.Run("Valid configuration", func(t *testing.T) {
		viper.Set("gcp.machines", []map[string]interface{}{
			{
				"location": "us-central1-a",
				"parameters": map[string]interface{}{
					"count":        1,
					"type":         "n1-standard-1",
					"orchestrator": true,
				},
			},
		})

		deployment, err := models.NewDeployment()
		assert.NoError(t, err)

		err = ProcessMachinesConfig(deployment)
		assert.NoError(t, err)
		assert.Len(t, deployment.Machines, 1)
	})

	// Test case 2: Invalid location
	t.Run("Invalid location", func(t *testing.T) {
		viper.Set("gcp.machines", []map[string]interface{}{
			{
				"location": "eu-west-1", // Invalid GCP location
				"parameters": map[string]interface{}{
					"count": 1,
					"type":  "n1-standard-1",
				},
			},
		})

		deployment, err := models.NewDeployment()
		assert.NoError(t, err)

		err = ProcessMachinesConfig(deployment)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid location for GCP: eu-west-1")
	})
}
