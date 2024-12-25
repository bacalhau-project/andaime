// Package sshutils provides interfaces for SSH operations
package sshutils

import (
	"io"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHSessioner represents an SSH session
type SSHSessioner interface {
	Run(cmd string) error
	Start(cmd string) error
	Wait() error
	Close() error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
}

// SSHClienter represents an SSH client
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
}

// SSHClientCreator creates SSH clients
type SSHClientCreator interface {
	NewClient(network, addr string, config *ssh.ClientConfig) (SSHClienter, error)
}

// SFTPClienter represents an SFTP client
type SFTPClienter interface {
	Create(path string) (sftp.File, error)
	Open(path string) (sftp.File, error)
	Remove(path string) error
	Close() error
}

// SFTPClientCreator creates SFTP clients
type SFTPClientCreator interface {
	NewClient(conn SSHClienter) (SFTPClienter, error)
}
