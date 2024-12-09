package sshutils

import "golang.org/x/crypto/ssh"

// SSHClienter defines the interface for SSH client operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
	GetClient() *ssh.Client
	IsConnected() bool
	Connect() (SSHClienter, error)
}

// SSHClientCreator defines the interface for creating SSH clients
type SSHClientCreator interface {
	NewClient(
		host string,
		port int,
		user string,
		privateKeyPath string,
		config *ssh.ClientConfig,
	) (SSHClienter, error)
}
