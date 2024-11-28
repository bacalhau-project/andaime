package aws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVPCIDToConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create test configuration
	config := []byte(`
deployments:
  aws:
    test-deployment:
      region: "us-west-2"
      instance_type: "t2.micro"
`)

	err := os.WriteFile(configFile, config, 0644)
	require.NoError(t, err)

	// Initialize viper with the test config
	viper.Reset()
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Create test deployment
	deployment := &models.Deployment{
		Name: "test-deployment",
		AWS: &models.AWSConfig{
			VPCID: "vpc-12345",
		},
	}

	// Write VPC ID to config
	err = writeVPCIDToConfig(deployment)
	require.NoError(t, err)

	// Verify the VPC ID was written correctly
	viper.ReadInConfig() // Reload config
	deploymentFromViper := viper.GetStringMap("deployments.aws")

	assert.Equal(
		t,
		deployment.AWS.VPCID,
		deploymentFromViper["test-deployment"].(map[string]interface{})["vpc_id"],
		"vpc_id should be written to config",
	)
}
