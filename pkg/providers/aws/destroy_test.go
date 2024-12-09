package aws

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
      region: "us-west-2"
      vpc_id: ""
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
	err = provider.Destroy(context.Background(), "")
	require.NoError(t, err)

	// Verify the deployment is removed from config
	viper.ReadInConfig() // Reload config
	deployments := viper.GetStringMap("deployments.aws")
	_, exists := deployments["test-deployment"]
	assert.False(t, exists, "test-deployment should be removed from config")
}
