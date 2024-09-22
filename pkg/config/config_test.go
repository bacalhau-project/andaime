package config

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testdata"
	internal_testutil "github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

type PkgConfigTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *PkgConfigTestSuite) SetupTest() {
	var err error
	suite.tempDir, err = os.MkdirTemp("", "config_test")
	suite.Require().NoError(err)
}

func (suite *PkgConfigTestSuite) TearDownTest() {
	os.RemoveAll(suite.tempDir)
}

func (suite *PkgConfigTestSuite) TestConfigFileReading() {
	testCases := []struct {
		name           string
		configContent  string
		expectedKeys   []string
		expectedError  bool
		validateConfig bool
	}{
		{
			name:          "SuccessfulReading",
			configContent: testdata.TestGenericConfig,
			expectedKeys:  []string{"general.project_prefix", "azure.subscription_id"},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			v := viper.New()

			configFile, cleanup, err := internal_testutil.WriteStringToTempFileWithExtension(
				tc.configContent,
				".yaml",
			)
			suite.Require().NoError(err)
			defer cleanup()

			v.SetConfigFile(configFile)

			err = v.ReadInConfig()
			if tc.expectedError {
				suite.Error(err, tc.name)
			} else {
				suite.NoError(err, tc.name)
				for _, key := range tc.expectedKeys {
					suite.True(v.IsSet(key), "Expected key %s not found in config", key)
				}
			}

			if tc.validateConfig {
				err = validateConfig(v)
				suite.Error(err, tc.name)
			}
		})
	}
}

func (suite *PkgConfigTestSuite) TestMinimalValidConfig() {
	tempPublicKey, err := os.CreateTemp(suite.tempDir, "id_rsa.pub")
	suite.Require().NoError(err)
	_, err = tempPublicKey.Write([]byte(testdata.TestPublicSSHKeyMaterial))
	suite.Require().NoError(err)
	tempPublicKey.Close()

	minimalConfig := fmt.Sprintf(`
general:
  project_prefix: "test-project"
  ssh_public_key_path: "%s"
azure:
  subscription_id: "test-subscription"
  resource_group: "test-group"
  locations:
    - name: "eastus"
      zones:
        - name: "1"
          machines:
            - type: "Standard_DS1_v2"
              count: 1
  allowed_ports:
    - 22
    - 80
    - 443
  disk_size_gb: 30
`, tempPublicKey.Name())

	v := viper.New()
	configFile, cleanup, err := internal_testutil.WriteStringToTempFileWithExtension(
		minimalConfig,
		".yaml",
	)
	suite.Require().NoError(err)
	defer cleanup()

	v.SetConfigFile(configFile)
	suite.NoError(v.ReadInConfig())

	suite.NoError(validateConfig(v))
	suite.NoError(validateConfigValues(v))
}

func (suite *PkgConfigTestSuite) TestLoadConfig() {
	testSSHPublicKeyPath, cleanupPublicKey, _, cleanupPrivateKey := internal_testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	testConfig := fmt.Sprintf(`
general:
  project_prefix: "test-project"
  ssh_public_key_path: "%s"		
aws:
  regions:
    - us-west-2
    - us-east-1
    - eu-west-1
`, testSSHPublicKeyPath)

	configFile, cleanup, err := internal_testutil.WriteStringToTempFile(testConfig)
	suite.Require().NoError(err)
	defer cleanup()

	config, err := LoadConfigFunc(configFile)
	suite.NoError(err)

	expectedRegions := []string{"us-west-2", "us-east-1", "eu-west-1"}
	suite.True(reflect.DeepEqual(config.GetStringSlice("aws.regions"), expectedRegions))
}

func (suite *PkgConfigTestSuite) TestLoadConfigError() {
	_, err := LoadConfig("non_existent_file.yaml")
	suite.Error(err)

	invalidConfig := `
aws:
  regions:
    - us-west-2
    - us-east-1
  : invalid-yaml
`
	configFile, cleanup, err := internal_testutil.WriteStringToTempFile(invalidConfig)
	suite.Require().NoError(err)
	defer cleanup()

	_, err = LoadConfig(configFile)
	suite.Error(err)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(PkgConfigTestSuite))
}
