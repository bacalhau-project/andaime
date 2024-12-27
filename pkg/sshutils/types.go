package sshutils

import (
	"time"
)

// DefaultTimeoutConfig returns the default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return DefaultTimeoutConfig()
}

type TimeoutConfig struct {
	SSHTimeout    time.Duration
	SFTPTimeout   time.Duration
	RetryInterval time.Duration
	MaxRetries    int
	WaitTimeout   time.Duration
}
