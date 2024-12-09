package sshutils

import (
	"fmt"

	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/pkg/sftp"
)

// defaultSFTPClientCreator is a struct for creating SFTP clients
type defaultSFTPClientCreator struct{}

func (d *defaultSFTPClientCreator) NewSFTPClient(
	client sshutils_interfaces.SSHClienter,
) (*sftp.Client, error) {
	if client == nil {
		return nil, fmt.Errorf("SSH client is nil")
	}

	sshClient := client.GetClient()
	if sshClient == nil {
		return nil, fmt.Errorf("SSH client connection is nil")
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return sftpClient, nil
}

// DefaultSFTPClientCreator is the default SFTP client creator instance
var DefaultSFTPClientCreator = &defaultSFTPClientCreator{}
