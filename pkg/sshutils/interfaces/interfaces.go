package interfaces

import (
"github.com/pkg/sftp"
"golang.org/x/crypto/ssh"
"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces/context"
"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces/types"
)

// SSHConfiger defines the interface for SSH configuration
type SSHConfiger interface {
types.SSHClientCreator
context.SSHOperations
}

// SFTPClientCreator defines the interface for creating SFTP clients
type SFTPClientCreator interface {
NewClient(sshClient *ssh.Client) (*sftp.Client, error)
}
