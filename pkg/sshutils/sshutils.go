package sshutils

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
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
	isPublicKey := len(path) > 4 && path[len(path)-4:] == ".pub" //nolint:mnd

	if isPublicKey {
		return []byte(testdata.TestPublicSSHKeyMaterial), nil
	} else {
		return []byte(testdata.TestPrivateSSHKeyMaterial), nil
	}
}

func ExtractSSHKeyPaths() (string, string, string, string, error) {
	publicKeyPath, err := extractSSHKeyPath("general.ssh_public_key_path")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to extract public key material: %w", err)
	}

	privateKeyPath, err := extractSSHKeyPath("general.ssh_private_key_path")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to extract private key material: %w", err)
	}

	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read public key file: %w", err)
	}

	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read private key file: %w", err)
	}

	return publicKeyPath, privateKeyPath, strings.TrimSpace(
			string(publicKeyData),
		), strings.TrimSpace(
			string(privateKeyData),
		), nil
}

func extractSSHKeyPath(configKeyString string) (string, error) {
	keyPath := viper.GetString(configKeyString)
	if keyPath == "" {
		return "", fmt.Errorf("%s is empty", configKeyString)
	}

	if strings.HasPrefix(keyPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[1:])
	}

	absoluteKeyPath, err := filepath.Abs(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for key file: %w", err)
	}

	if _, err := os.Stat(absoluteKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("key file does not exist: %s", absoluteKeyPath)
	}

	return absoluteKeyPath, nil
}

func ReadPrivateKey(path string) ([]byte, error) {
	privateKeyFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open private key file: %w", err)
	}
	defer privateKeyFile.Close()

	privateKeyBytes, err := io.ReadAll(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	if _, err = ssh.ParsePrivateKey(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKeyBytes, nil
}

func ReadPublicKey(path string) ([]byte, error) {
	publicKeyFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open public key file: %w", err)
	}
	defer publicKeyFile.Close()

	publicKeyBytes, err := io.ReadAll(publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	return publicKeyBytes, nil
}

func GenerateRsaKeyPair() (*rsa.PrivateKey, *rsa.PublicKey) {
	privkey, _ := rsa.GenerateKey(rand.Reader, 4096)
	return privkey, &privkey.PublicKey
}

func ExportRsaPrivateKeyAsPemStr(privkey *rsa.PrivateKey) string {
	privkeyBytes := x509.MarshalPKCS1PrivateKey(privkey)
	privkeyPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privkeyBytes,
		},
	)
	return string(privkeyPem)
}

func ParseRsaPrivateKeyFromPemStr(privPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return priv, nil
}

func ExportRsaPublicKeyAsPemStr(pubkey *rsa.PublicKey) (string, error) {
	pubkeyBytes, err := x509.MarshalPKIXPublicKey(pubkey)
	if err != nil {
		return "", err
	}
	pubkeyPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pubkeyBytes,
		},
	)

	return string(pubkeyPem), nil
}

func ParseRsaPublicKeyFromPemStr(pubPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	switch pub := pub.(type) {
	case *rsa.PublicKey:
		return pub, nil
	default:
		break // fall through
	}
	return nil, errors.New("key type is not RSA")
}
