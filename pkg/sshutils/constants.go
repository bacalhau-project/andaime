package sshutils

import "time"

var (
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 1 * time.Minute
	SSHRetryAttempts        = 3
	SSHDialTimeout          = 3 * time.Second
	SSHClientConfigTimeout  = 3 * time.Second
	SSHRetryDelay           = 3 * time.Second
)

func GetAggregateSSHTimeout() time.Duration {
	totalTimeout := (SSHTimeOut + TimeInBetweenSSHRetries + SSHRetryDelay) * time.Duration(
		SSHRetryAttempts,
	)
	return totalTimeout
}
