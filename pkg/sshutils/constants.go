package sshutils

import (
	"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces/types"
)

var (
	TimeInBetweenSSHRetries = 2 * types.Second
	SSHTimeOut              = 1 * types.Minute
	SSHRetryAttempts        = 3
	SSHDialTimeout          = 10 * types.Second
	SSHClientConfigTimeout  = 10 * types.Second
	SSHRetryDelay           = 20 * types.Second
)

func GetAggregateSSHTimeout() types.Duration {
	totalTimeout := (SSHTimeOut + TimeInBetweenSSHRetries + SSHRetryDelay) * types.Duration(
		SSHRetryAttempts,
	)
	return totalTimeout
}
