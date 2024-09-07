package sshutils

import "time"

var (
	NumberOfSSHRetries      = 3
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 30 * time.Second
	SSHRetryAttempts        = 6
	SSHRetryDelay           = 10 * time.Second
)

func GetAggregateSSHTimeout() time.Duration {
	totalTimeout := (SSHTimeOut + TimeInBetweenSSHRetries + SSHRetryDelay) * time.Duration(
		SSHRetryAttempts,
	)
	return totalTimeout
}
