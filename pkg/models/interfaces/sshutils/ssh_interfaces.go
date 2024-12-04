package sshutils

import (
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHSessioner defines the interface for SSH session operations
type SSHSessioner interface {
	Run(cmd string) error
	CombinedOutput(cmd string) ([]byte, error)
	Close() error
	StdinPipe() (io.WriteCloser, error)
	Start(cmd string) error
	Wait() error
}

// SSHClienter defines the interface for SSH client operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
	GetClient() *ssh.Client
}
