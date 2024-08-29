package sshutils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func retry(attempts int, sleep time.Duration, f func() error) error {
	var err error
	for i := 0; i < SSHRetryAttempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %w", attempts, err)
}

func getHostKey(host string) (ssh.PublicKey, error) {
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("failed to open known_hosts file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		numberOfPublicKeyFields := 3
		if len(fields) != numberOfPublicKeyFields {
			continue
		}
		if strings.Contains(fields[0], host) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				return nil, fmt.Errorf("failed to parse host key: %w", err)
			}
			break
		}
	}

	if hostKey == nil {
		return nil, fmt.Errorf("no hostkey found for %s", host)
	}

	return hostKey, nil
}

func GetHostKeyCallback(host string) (ssh.HostKeyCallback, error) {
	hostKey, err := getHostKey(host)
	if err != nil {
		return nil, err
	}
	return ssh.FixedHostKey(hostKey), nil
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
func ValidateSSHKeysFromPath(publicKeyPath, privateKeyPath string) error {
	// Read public key
	log := logger.Get()
	log.Debugf("Reading public key from path: %s", publicKeyPath)
	publicKey, err := SSHKeyReader(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Read private key
	log.Debugf("Reading private key from path: %s", privateKeyPath)
	privateKey, err := SSHKeyReader(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	// Validate public key
	if !isValidPublicKey(string(publicKey)) {
		return fmt.Errorf("invalid public key format")
	}

	// Validate private key
	if !isValidPrivateKey(string(privateKey)) {
		return fmt.Errorf("invalid private key format")
	}

	return nil
}

func isValidPublicKey(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "ssh-")
}

func isValidPrivateKey(key string) bool {
	return strings.Contains(key, "BEGIN") && strings.Contains(key, "PRIVATE KEY")
}

var MockSSHKeyReader = func(path string) ([]byte, error) {
	// if the key path ends in .pub, we assume it's the public key
	isPublicKey := len(path) > 4 && path[len(path)-4:] == ".pub" //nolint:gomnd

	if isPublicKey {
		return []byte(testdata.TestPublicSSHKeyMaterial), nil
	} else {
		return []byte(testdata.TestPrivateSSHKeyMaterial), nil
	}
}
