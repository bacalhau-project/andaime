package sshutils

import "io"

// SSHSessioner defines the interface for SSH session operations
type SSHSessioner interface {
	Run(cmd string) error
	CombinedOutput(cmd string) ([]byte, error)
	Close() error
	StdinPipe() (io.WriteCloser, error)
	Start(cmd string) error
	Wait() error
}
