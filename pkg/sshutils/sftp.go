package sshutils

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
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

// SFTPClientCreator is a function type for creating SFTP clients
type SFTPClientCreator func(conn *ssh.Client) (SFTPClienter, error)

// defaultSFTPClient wraps the sftp.Client to implement our interface
type defaultSFTPClient struct {
	*sftp.Client
}

func (c *defaultSFTPClient) MkdirAll(path string) error {
	return c.Client.MkdirAll(path)
}

// Create creates a new file on the remote system
func (c *defaultSFTPClient) Create(path string) (io.WriteCloser, error) {
	// Ensure parent directory exists
	if err := c.MkdirAll(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return c.Client.Create(path)
}

func (c *defaultSFTPClient) Open(path string) (io.ReadCloser, error) {
	return c.Client.Open(path)
}

// NewSFTPClient creates a new SFTP client from an SSH client
func NewSFTPClient(client *ssh.Client) (SFTPClienter, error) {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	return &defaultSFTPClient{Client: sftpClient}, nil
}

// Default SFTP client creator
var DefaultSFTPClientCreator SFTPClientCreator = NewSFTPClient
