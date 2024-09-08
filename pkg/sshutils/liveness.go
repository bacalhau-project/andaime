package sshutils

import (
	"time"
)

const (
	DefaultSSHPort    = 22
	DefaultRetryCount = 5
	DefaultRetryDelay = 10 * time.Second
)
