package sshutils

import (
	"golang.org/x/crypto/ssh"
)

func (s *SSHSessionWrapper) Signal(sig ssh.Signal) error {
	return s.Session.Signal(sig)
}
