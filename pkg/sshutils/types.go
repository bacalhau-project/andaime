package sshutils

import (
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
)

// DefaultTimeoutConfig returns the default timeout configuration
func DefaultTimeoutConfig() sshutils_interfaces.TimeoutConfig {
	return sshutils_interfaces.DefaultTimeoutConfig()
}
