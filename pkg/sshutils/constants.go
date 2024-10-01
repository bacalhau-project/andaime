package sshutils

import "time"

var (
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 1 * time.Minute
	SSHRetryAttempts        = 5
	SSHRetryDelay           = 10 * time.Second
)

func GetAggregateSSHTimeout() time.Duration {
	totalTimeout := (SSHTimeOut + TimeInBetweenSSHRetries + SSHRetryDelay) * time.Duration(
		SSHRetryAttempts,
	)
	return totalTimeout
}
