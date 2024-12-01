package sshutils

import "golang.org/x/crypto/ssh"

// SSHClienter defines the interface for SSH client operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
	GetClient() *ssh.Client
}
