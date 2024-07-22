package testutil

import (
	"os"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/spf13/viper"
)

func GetTestAzureViper() (*viper.Viper, error) {
	viper.Reset()
	testConfig := viper.New()
	configFile, cleanup, err := WriteStringToTempFile(testdata.TestAzureConfig)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	testConfig.SetConfigType("yaml")
	testConfig.SetConfigFile(configFile)
	err = testConfig.ReadInConfig()
	if err != nil {
		return nil, err
	}
	return testConfig, nil
}

// returns the file path and a cleanup function.
func WriteStringToTempFileWithExtension(content string, extension string) (string, func(), error) {
	path, cleanup, err := WriteStringToTempFile(content)
	if err != nil {
		return "", nil, err
	}

	return path + extension, cleanup, nil
}

func WriteStringToTempFile(content string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "temp-*")
	if err != nil {
		return "", nil, err
	}

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", nil, err
	}

	tempFile.Close()

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), cleanup, nil
}

func CreateSSHPublicPrivateKeyPairOnDisk() (string, func(), string, func()) {
	testSSHPublicKeyPath, cleanupPublicKey, err := WriteStringToTempFile(testdata.TestPublicSSHKeyMaterial)
	if err != nil {
		panic(err)
	}
	testSSHPrivateKeyPath, cleanupPrivateKey, err := WriteStringToTempFile(testdata.TestPrivateSSHKeyMaterial)
	if err != nil {
		panic(err)
	}

	return testSSHPublicKeyPath, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey
}
