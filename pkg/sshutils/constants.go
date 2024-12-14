package sshutils

import "time"

var (
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 1 * time.Minute
	SSHRetryAttempts        = 3
	SSHDialTimeout          = 10 * time.Second
	SSHClientConfigTimeout  = 10 * time.Second
	SSHRetryDelay           = 20 * time.Second
)

func GetAggregateSSHTimeout() time.Duration {
	totalTimeout := (SSHTimeOut + TimeInBetweenSSHRetries + SSHRetryDelay) * time.Duration(
		SSHRetryAttempts,
	)
	return totalTimeout
}
