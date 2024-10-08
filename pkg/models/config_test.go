package models

import (
	"bytes"
	"os"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadBacalhauSettings(t *testing.T) {
	t.Run("Valid Azure Config with Bacalhau Settings", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "bacalhau_settings_*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		testAzureConfig, err := testdata.ReadTestAzureConfig()
		require.NoError(t, err)

		err = os.WriteFile(tempFile.Name(), []byte(testAzureConfig), 0600)
		require.NoError(t, err)

		viper.SetConfigFile(tempFile.Name())
		err = viper.ReadInConfig()
		require.NoError(t, err)

		settings, err := ReadBacalhauSettingsFromViper()
		assert.NoError(t, err)
		assert.NotNil(t, settings)

		// Go through each string and see if there is a BacalhauSettings that has it as the Key

		listOfKeys := []string{
			"compute.allowlistedlocalpaths",
			"compute.heartbeat.interval",
			"compute.heartbeat.infoupdateinterval",
			"compute.heartbeat.resourceupdateinterval",
			"orchestrator.nodemanager.disconnecttimeout",
			"jobadmissioncontrol.acceptnetworkedjobs",
		}

		for _, key := range listOfKeys {
			found := false
			for _, setting := range settings {
				if setting.Key == key {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected key %s not found in Bacalhau settings", key)
		}

		for _, setting := range settings {
			if setting.Key == "compute.allowlistedlocalpaths" {
				paths, ok := setting.Value.([]string)
				assert.True(t, ok)
				assert.Contains(t, paths, "/tmp")
				assert.Contains(t, paths, "/data")
			}
		}
	})

	t.Run("Empty Bacalhau Settings", func(t *testing.T) {
		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBufferString(`
provider: azure
general:
  bacalhau_settings: {}
`))
		require.NoError(t, err)

		settings, err := ReadBacalhauSettingsFromViper()
		assert.NoError(t, err)
		assert.Empty(t, settings)
	})

	t.Run("Invalid Bacalhau Settings Type", func(t *testing.T) {
		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBufferString(`
provider: azure
general:
  bacalhau_settings: "not a map"
`))
		require.NoError(t, err)

		settings, err := ReadBacalhauSettingsFromViper()
		assert.Error(t, err)
		assert.Nil(t, settings)
	})

	t.Run("Invalid Value Type for Bacalhau Setting", func(t *testing.T) {
		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBufferString(`
provider: azure
general:
  bacalhau_settings:
    compute.allowlistedlocalpaths: {"mapsNotSupported": "b", "c": "d"}
`))
		require.NoError(t, err)

		settings, err := ReadBacalhauSettingsFromViper()
		assert.Error(t, err)
		assert.Nil(t, settings)
	})

	t.Run("Invalid Bacalhau Settings", func(t *testing.T) {
		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBufferString(`
provider: azure
general:
  bacalhau_settings:
    BAD_KEY: BAD_VALUE
`))
		require.NoError(t, err)

		settings, err := ReadBacalhauSettingsFromViper()
		assert.Error(t, err)
		assert.Nil(t, settings)
	})
}
