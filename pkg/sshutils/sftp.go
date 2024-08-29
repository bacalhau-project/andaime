package sshutils

import (
	"io"

	"golang.org/x/crypto/ssh"
)

// SFTPClienter interface defines the methods we need for SFTP operations
type SFTPClienter interface {
	Create(path string) (io.WriteCloser, error)
	Open(path string) (io.ReadCloser, error)
	Close() error
}

// SFTPDial is a function type for creating SFTP clients
type SFTPDialFunc func(conn *ssh.Client) (SFTPClienter, error)
