package sshutils

import (
	"context"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfiger defines the interface for SSH configuration and operations
type SSHConfiger interface {
	// Connection methods
	Connect() (SSHClienter, error)
	WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error
	Close() error

	// Configuration getters
	GetHost() string
	GetPort() int
	GetUser() string
	GetPrivateKeyMaterial() []byte

	// SSH client management
	GetSSHClient() *ssh.Client
	SetSSHClient(client *ssh.Client)
	SetSSHClienter(client SSHClienter)
	SetValidateSSHConnection(fn func() error)

	// Remote operations
	ExecuteCommand(ctx context.Context, command string) (string, error)
	ExecuteCommandWithCallback(
		ctx context.Context,
		command string,
		callback func(string),
	) (string, error)
	PushFile(ctx context.Context, remotePath string, content []byte, executable bool) error
	PushFileWithCallback(
		ctx context.Context,
		remotePath string,
		content []byte,
		executable bool,
		callback func(int64, int64),
	) error

	// Service management
	InstallSystemdService(ctx context.Context, serviceName, serviceContent string) error
	StartService(ctx context.Context, serviceName string) (string, error)
	RestartService(ctx context.Context, serviceName string) (string, error)
}