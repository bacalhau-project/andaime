package sshutils

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// SSHSessioner is now defined in interfaces.go

// ValidateSSHPublicKey checks if the provided SSH public key is valid
func ValidateSSHPublicKey(key string) error {
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return fmt.Errorf("invalid SSH public key format")
	}

	// Check if the key type is supported (ssh-rsa, ssh-ed25519, etc.)
	keyType := parts[0]
	supportedTypes := []string{
		"ssh-rsa",
		"ssh-ed25519",
		"ecdsa-sha2-nistp256",
		"ecdsa-sha2-nistp384",
		"ecdsa-sha2-nistp521",
	}
	isSupported := false
	for _, t := range supportedTypes {
		if keyType == t {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return fmt.Errorf("unsupported SSH public key type: %s", keyType)
	}

	// Check if the key data is base64 encoded
	keyData := parts[1]
	if _, err := base64.StdEncoding.DecodeString(keyData); err != nil {
		return fmt.Errorf("invalid SSH public key: key data is not base64 encoded")
	}

	return nil
}
