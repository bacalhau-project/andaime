package utils

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	NumberOfSSHRetries      = 3
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 30 * time.Second
)

type Config struct {
	AWS struct {
		Regions []string `yaml:"regions"`
	} `yaml:"aws"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

var SSHKeyReader = func(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// GetSSHKeysFromPath reads and returns the public and private SSH keys from the specified path.
// If the path ends with ".pub", it is assumed to be the public key, and the corresponding private key
// is expected to be located at the same path without the ".pub" extension. The function uses SSHKeyReader
// to read the keys from the filesystem.
//
// Parameters:
// - path: The filesystem path to the public key or the base path for both keys if the path does not end with ".pub".
//
// Returns:
// - The public key as the first return value.
// - The private key as the second return value.
// - An error if either key could not be read, otherwise nil.
//
// Note: The function assumes that the public and private keys are named identically with the only difference
// being the ".pub" extension for the public key.
func GetSSHKeysFromPath(path string) ([]byte, []byte, error) {
	// If the key path ends in .pub, we assume it's the public key
	// and we need to remove the .pub extension to get the private key path

	if len(path) > 4 && path[len(path)-4:] == ".pub" {
		path = path[:len(path)-4]
	}

	privateKeyPath := path
	publicKeyPath := fmt.Sprintf("%s.pub", path)

	privateKey, err := SSHKeyReader(privateKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read private key: %w", err)
	}

	publicKey, err := SSHKeyReader(publicKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read public key: %w", err)
	}

	return publicKey, privateKey, nil
}

//nolint:lll
const TestPublicSSHKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC4VNbBAdUjsEGtthi6f804ftcSer2BUHJ4n4I2olBOB dummy@example.com"
const TestPrivateSSHKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAuFTWwQHVI7BBrbYYun/NOH7XEnq9gVByeJ+CNqJQTgQAAAJg1FTcNNRU3
DQAAAAtzc2gtZWQyNTUxOQAAACAuFTWwQHVI7BBrbYYun/NOH7XEnq9gVByeJ+CNqJQTgQ
AAAEAiSKPZOlligMHdH5BZdobDWhuyMkR+mR/s16zklfhFii4VNbBAdUjsEGtthi6f804f
tcSer2BUHJ4n4I2olBOBAAAAEWR1bW15QGV4YW1wbGUuY29tAQIDBA==
-----END OPENSSH PRIVATE KEY-----`

var MockSSHKeyReader = func(path string) ([]byte, error) {
	// if the key path ends in .pub, we assume it's the public key
	isPublicKey := len(path) > 4 && path[len(path)-4:] == ".pub" //nolint:gomnd

	if isPublicKey {
		return []byte(TestPublicSSHKey), nil
	} else {
		return []byte(TestPrivateSSHKey), nil
	}
}
