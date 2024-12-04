package sshutils

import (
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

// SFTPClienter interface defines the methods we need for SFTP operations
type SFTPClienter interface {
	Create(path string) (io.WriteCloser, error)
	Open(path string) (io.ReadCloser, error)
	MkdirAll(path string) error
	Chmod(path string, mode os.FileMode) error
	Close() error
}

type SFTPClientCreator func(client *ssh.Client) (SFTPClienter, error)
