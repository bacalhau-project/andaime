package common

import "time"

const (
	UpdateQueueSize         = 100
	ResourcePollingInterval = 2 * time.Second
	DebugFilePath           = "/tmp/andaime-debug.log"
	DefaultSSHUser          = "andaimeuser"
	DebugFilePermissions    = 0644
	WaitingForMachinesTime  = 1 * time.Minute
)
