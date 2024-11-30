package sshutils

import (
	"context"
	"fmt"
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

	// Dialer configuration
	GetSSHDial() SSHDialer
	SetSSHDial(dialer SSHDialer)
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
	StartService(ctx context.Context, serviceName string) error
	RestartService(ctx context.Context, serviceName string) error
	RestartService(ctx context.Context, serviceName string) error
}

// SSHClienter defines the interface for SSH client operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
}

// SSHSessioner defines the interface for SSH session operations
type SSHSessioner interface {
	Run(cmd string) error
	CombinedOutput(cmd string) ([]byte, error)
	Close() error
}

// SSHDialer defines the interface for SSH dialing operations
type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error)
	DialContext(
		ctx context.Context,
		network, addr string,
		config *ssh.ClientConfig,
	) (SSHClienter, error)
}

// SSHClientWrapper wraps an ssh.Client
type SSHClientWrapper struct {
	Client *ssh.Client
}

func (w *SSHClientWrapper) Close() error {
	if w.Client != nil {
		return w.Client.Close()
	}
	return nil
}

// SSHSessionWrapper wraps an ssh.Session
type SSHSessionWrapper struct {
	Session *ssh.Session
}

// SSHError represents an SSH command execution error with output
type SSHError struct {
	Cmd    string
	Output string
	Err    error
}

func (e *SSHError) Error() string {
	return fmt.Sprintf("SSH command failed:\nCommand: %s\nOutput: %s\nError: %v",
		e.Cmd, e.Output, e.Err)
}
