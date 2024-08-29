package sshutils

import "time"

var (
	NumberOfSSHRetries      = 3
	TimeInBetweenSSHRetries = 2 * time.Second
	SSHTimeOut              = 30 * time.Second
	SSHRetryAttempts        = 30
	SSHRetryDelay           = 10 * time.Second
)
