package awsprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestroyWithEmptyVPCID(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create test configuration
	config := []byte(`
deployments:
  aws:
    test-deployment:
      vpc_id: ""
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

	// Create AWS provider
	provider := &AWSProvider{}

	// Call destroy
	err = provider.DestroyResources(context.Background(), "")
	assert.NoError(t, err)

	// Verify the VPC ID is removed from config
	viper.ReadInConfig() // Reload config
	deployments := viper.GetStringMap("deployments.aws")
	deployment := deployments["test-deployment"].(map[string]interface{})
	_, exists := deployment["vpc_id"]
	assert.False(t, exists, "vpc_id should be removed from config")
}
