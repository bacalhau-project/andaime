package sshutils

import "time"

var (
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 30 * time.Second
	SSHRetryAttempts        = 3
	SSHRetryDelay           = 5 * time.Second
)

func GetAggregateSSHTimeout() time.Duration {
	totalTimeout := (SSHTimeOut + TimeInBetweenSSHRetries + SSHRetryDelay) * time.Duration(
		SSHRetryAttempts,
	)
	return totalTimeout
}
