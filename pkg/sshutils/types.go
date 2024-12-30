package sshutils

import (
	"time"
)

// DefaultTimeoutConfig returns the default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		SSHTimeout:    SSHTimeOut,
		SFTPTimeout:   SSHRetryDelay,
		RetryInterval: TimeInBetweenSSHRetries,
		MaxRetries:    SSHRetryAttempts,
		WaitTimeout:   SSHRetryDelay * time.Duration(SSHRetryAttempts),
	}
}

type TimeoutConfig struct {
	SSHTimeout    time.Duration
	SFTPTimeout   time.Duration
	RetryInterval time.Duration
	MaxRetries    int
	WaitTimeout   time.Duration
}
