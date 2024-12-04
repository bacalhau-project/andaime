package sshutils

import (
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
)

// Ensure we implement the SSHClienter and SSHSessioner interfaces
var (
	_ sshutils_interfaces.SSHClienter  = &SSHClientWrapper{}
	_ sshutils_interfaces.SSHSessioner = &SSHSessionWrapper{}
)
