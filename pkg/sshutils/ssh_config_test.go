package sshutils

import (
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestSSHConfigValidation(t *testing.T) {
	_,
		cleanupPrivateKey,
		testPrivateKeyPath,
		cleanupPublicKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPrivateKey()
	defer cleanupPublicKey()

	tests := []struct {
		name          string
		host          string
		port          int
		user          string
		keyPath       string
		expectedError string
	}{
		{
			name:          "valid config",
			host:          "example.com",
			port:          22,
			user:          "testuser",
			keyPath:       testPrivateKeyPath,
			expectedError: "",
		},
		{
			name:          "empty host",
			host:          "",
			port:          22,
			user:          "testuser",
			keyPath:       testPrivateKeyPath,
			expectedError: "host cannot be empty",
		},
		{
			name:          "invalid port",
			host:          "example.com",
			port:          0,
			user:          "testuser",
			keyPath:       testPrivateKeyPath,
			expectedError: "invalid port number: 0",
		},
		{
			name:          "empty user",
			host:          "example.com",
			port:          22,
			user:          "",
			keyPath:       testPrivateKeyPath,
			expectedError: "user cannot be empty",
		},
		{
			name:          "empty key path",
			host:          "example.com",
			port:          22,
			user:          "testuser",
			keyPath:       "",
			expectedError: "private key path is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with mocked key reader
			config, err := NewSSHConfigFunc(tt.host, tt.port, tt.user, tt.keyPath)
			if tt.expectedError != "" {
				if err == nil {
					// If validation function exists, test it directly
					if config != nil {
						err = config.(*SSHConfig).ValidateSSHConnectionFunc()
					}
				}
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				// Test validation function
				err = config.(*SSHConfig).ValidateSSHConnectionFunc()
				assert.NoError(t, err)
			}
		})
	}
}
