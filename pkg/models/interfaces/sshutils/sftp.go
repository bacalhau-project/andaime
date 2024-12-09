package sshutils

import (
	"io"
	"os"
)

//go:generate mockery --name SFTPFile
type SFTPFile interface {
	Write(p []byte) (n int, err error)
	Close() error
}

// SFTPClienter defines the interface for SFTP clients
type SFTPClienter interface {
	Create(string) (io.WriteCloser, error)
	Open(string) (io.ReadCloser, error)
	MkdirAll(string) error
	Chmod(string, os.FileMode) error
	Close() error
}
