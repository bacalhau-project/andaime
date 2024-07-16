package sshutils

import (
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHClient interface defines the methods we need for SSH operations
type SSHClient interface {
	NewSession() (SSHSession, error)
	Close() error
}

// SSHSession interface defines the methods we need for SSH sessions
type SSHSession interface {
	Run(cmd string) error
	CombinedOutput(cmd string) ([]byte, error)
	Close() error
}

// SFTPClient interface defines the methods we need for SFTP operations
type SFTPClient interface {
	Create(path string) (io.WriteCloser, error)
	Open(path string) (io.ReadCloser, error)
	Close() error
}

// SSHDial is a function type for creating SSH clients
type SSHDial func(network, addr string, config *ssh.ClientConfig) (SSHClient, error)

// SFTPDial is a function type for creating SFTP clients
type SFTPDial func(conn *ssh.Client) (SFTPClient, error)
