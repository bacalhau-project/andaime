package testdata

import _ "embed"

//go:embed dummy_keys/id_ed25519.pub
var TestPublicSSHKeyMaterial string

func ReadTestPublicSSHKeyMaterial() (string, error) {
	return TestPublicSSHKeyMaterial, nil
}

//go:embed dummy_keys/id_ed25519
var TestPrivateSSHKeyMaterial string

func ReadTestPrivateSSHKeyMaterial() (string, error) {
	return TestPrivateSSHKeyMaterial, nil
}

//go:embed configs/azure.yaml
var TestAzureConfig string

func ReadTestAzureConfig() (string, error) {
	return TestAzureConfig, nil
}

//go:embed configs/gcp.yaml
var TestGCPConfig string

func ReadTestGCPConfig() (string, error) {
	return TestGCPConfig, nil
}

//go:embed configs/aws.yaml
var TestAWSConfig string

func ReadTestAWSConfig() (string, error) {
	return TestAWSConfig, nil
}

//go:embed configs/config.yaml
var TestGenericConfig string

func ReadTestGenericConfig() (string, error) {
	return TestGenericConfig, nil
}
