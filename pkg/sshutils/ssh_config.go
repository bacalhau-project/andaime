package sshutils

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var NullCallback = func(int64, int64) {}

type SSHConfig struct {
	Host               string
	Port               int
	User               string
	SSHPrivateKeyPath  string
	PrivateKeyMaterial []byte
	Timeout            time.Duration
	Logger             *logger.Logger

	SSHClient SSHClienter
	SSHDial   SSHDialer

	SSHPrivateKeyReader   func(path string) ([]byte, error)
	SSHPublicKeyReader    func(path string) ([]byte, error)
	ClientConfig          *ssh.ClientConfig
	InsecureIgnoreHostKey bool
}

type SSHConfiger interface {
	SetSSHClient(client SSHClienter)
	Connect() (SSHClienter, error)
	WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error
	ExecuteCommand(ctx context.Context, command string) (string, error)
	ExecuteCommandWithCallback(ctx context.Context,
		command string,
		callback func(string)) (string, error)
	PushFile(ctx context.Context, remotePath string, content []byte, executable bool) error
	PushFileWithCallback(ctx context.Context,
		remotePath string,
		content []byte,
		executable bool,
		callback func(int64, int64)) error
	InstallSystemdService(ctx context.Context, serviceName, serviceContent string) error
	StartService(ctx context.Context, serviceName string) error
	RestartService(ctx context.Context, serviceName string) error
}

var NewSSHConfigFunc = NewSSHConfig

func NewSSHConfig(
	host string,
	port int,
	user string,
	sshPrivateKeyPath string,
) (SSHConfiger, error) {
	if len(sshPrivateKeyPath) == 0 {
		return nil, fmt.Errorf("private key path is empty")
	}

	if len(host) == 0 {
		return nil, fmt.Errorf("host is empty")
	}

	if len(user) == 0 {
		return nil, fmt.Errorf("user is empty")
	}

	if port == 0 {
		return nil, fmt.Errorf("port is empty")
	}

	sshClientConfig, err := getSSHClientConfig(user, host, sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH client config: %w", err)
	}

	sshPrivateKeyMaterial, err := os.ReadFile(sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key material: %w", err)
	}

	dialer := NewSSHDial(host, port, sshClientConfig)

	return &SSHConfig{
		Host:                  host,
		Port:                  port,
		User:                  user,
		SSHPrivateKeyPath:     sshPrivateKeyPath,
		PrivateKeyMaterial:    sshPrivateKeyMaterial,
		Timeout:               SSHTimeOut,
		Logger:                logger.Get(),
		ClientConfig:          sshClientConfig,
		SSHDial:               dialer,
		InsecureIgnoreHostKey: false,
	}, nil
}

func getSSHClientConfig(user, host, privateKeyPath string) (*ssh.ClientConfig, error) {
	l := logger.Get()
	l.Debugf("Getting SSH client config for %s", host)

	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	key, err := getPrivateKey(string(privateKeyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %v", err)
	}

	// Use a custom host key callback that ignores mismatches and insecure connections
	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// This callback accepts all host keys
		return nil
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         SSHClientConfigTimeout,
	}, nil
}

func getPrivateKey(privateKeyMaterial string) (ssh.Signer, error) {
	// Check if the key material is empty
	if len(privateKeyMaterial) == 0 {
		return nil, fmt.Errorf("SSH private key is empty")
	}

	// Attempt to parse the private key
	privateKey, err := ssh.ParsePrivateKey([]byte(privateKeyMaterial))
	if err != nil {
		// Provide more detailed error messages for common key parsing issues
		switch {
		case strings.Contains(err.Error(), "x509: malformed private key"):
			return nil, fmt.Errorf("invalid private key format: malformed key")
		case strings.Contains(err.Error(), "x509: unsupported key type"):
			return nil, fmt.Errorf("unsupported private key type")
		case strings.Contains(err.Error(), "x509: key is encrypted"):
			return nil, fmt.Errorf("encrypted private key is not supported: remove passphrase")
		case strings.Contains(err.Error(), "failed to parse private key"):
			return nil, fmt.Errorf("failed to parse private key: incorrect format or permissions")
		default:
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
	}

	// Additional validation
	if privateKey == nil {
		return nil, fmt.Errorf("parsed private key is nil")
	}

	// Check key type
	switch privateKey.PublicKey().Type() {
	case "ssh-rsa", "ssh-ed25519", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		// Supported key types
	default:
		return nil, fmt.Errorf("unsupported SSH key type: %s", privateKey.PublicKey().Type())
	}

	return privateKey, nil
}

func (c *SSHConfig) SetSSHClient(client SSHClienter) {
	c.SSHClient = client
}

func (c *SSHConfig) validateSSHConnection() error {
	l := logger.Get()

	// Validate host and port
	if c.Host == "" {
		return fmt.Errorf("host is empty")
	}

	if c.Port == 0 {
		return fmt.Errorf("port is empty")
	}

	// Check if port is open
	address := fmt.Sprintf("%s:%d", c.Host, c.Port)
	conn, err := net.DialTimeout("tcp", address, SSHDialTimeout)
	if err != nil {
		l.Debugf("Port %d is closed on host %s", c.Port, c.Host)
		return fmt.Errorf("SSH port %d is closed on host %s: %v", c.Port, c.Host, err)
	}
	defer conn.Close()

	// Validate SSH private key
	if _, err := getPrivateKey(string(c.PrivateKeyMaterial)); err != nil {
		l.Debugf("Invalid SSH private key: %v", err)
		return fmt.Errorf("invalid SSH private key: %v", err)
	}

	return nil
}

func (c *SSHConfig) Connect() (SSHClienter, error) {
	l := logger.Get()
	l.Infof("Connecting to SSH server: %s:%d", c.Host, c.Port)

	// Validate connection prerequisites
	if err := c.validateSSHConnection(); err != nil {
		return nil, err
	}

	var err error
	var client SSHClienter

	for i := 0; i < SSHRetryAttempts; i++ {
		l.Debugf("Attempt %d to connect via SSH\n", i+1)
		client, err = c.SSHDial.Dial(
			"tcp",
			fmt.Sprintf("%s:%d", c.Host, c.Port),
			c.ClientConfig,
		)
		if err == nil {
			break
		}

		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			l.Debugf("timed out waiting to connect to SSH server\n")
			continue
		}

		l.Error(err.Error())

		time.Sleep(SSHRetryDelay)
	}

	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, fmt.Errorf(
				"SSH connection refused: check host, port, and firewall settings",
			)
		}

		if strings.Contains(err.Error(), "no route to host") {
			return nil, fmt.Errorf("no route to host: verify network connectivity")
		}

		if strings.Contains(err.Error(), "handshake failed") {
			return nil, fmt.Errorf(
				"SSH handshake failed: check username and private key authentication",
			)
		}

		return nil, fmt.Errorf("SSH connection failed: %v", err)
	}

	c.SSHClient = client

	return client, nil
}

func (c *SSHConfig) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	l := logger.Get()
	for i := 0; i < SSHRetryAttempts; i++ {
		l.Debugf("Attempt %d to connect via SSH\n", i+1)
		client, err := c.Connect()
		if err != nil {
			err = fmt.Errorf("failed to connect to SSH: %v", err)
			l.Error(err.Error())
			time.Sleep(SSHRetryDelay)
			continue
		}
		session, err := client.NewSession()
		if err != nil {
			err = fmt.Errorf("failed to create SSH session: %v", err)
			l.Error(err.Error())
			client.Close()
			time.Sleep(SSHRetryDelay)
			continue
		}

		defer func() {
			if client != nil {
				client.Close()
			}
			if session != nil {
				session.Close()
			}
		}()

		if session == nil {
			err = fmt.Errorf("SSH session is nil despite no error")
			l.Error(err.Error())
			time.Sleep(SSHRetryDelay)
			continue
		}

		l.Debug("SSH connection established")
		return nil
	}

	err := fmt.Errorf("failed to establish SSH connection after multiple attempts")
	l.Error(err.Error())
	return err
}

func (c *SSHConfig) NewSession() (SSHSessioner, error) {
	if c.SSHClient == nil {
		sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), c.ClientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH client: %w", err)
		}
		c.SSHClient = &SSHClientWrapper{Client: sshClient}
	}

	if !c.SSHClient.IsConnected() {
		connectedClient, err := c.Connect()
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SSH: %w", err)
		}
		c.SSHClient.(*SSHClientWrapper).Client = connectedClient.(*SSHClientWrapper).Client
	}

	return c.SSHClient.NewSession()
}

func (c *SSHConfig) ExecuteCommand(ctx context.Context, command string) (string, error) {
	return c.ExecuteCommandWithCallback(ctx, command, nil)
}

// ExecuteCommandWithCallback runs a command on the remote server over SSH.
// It takes the command as a string argument.
// It retries the execution a configurable number of times if it fails.
// It returns the output of the command as a string and any error encountered.
func (c *SSHConfig) ExecuteCommandWithCallback(ctx context.Context,
	command string,
	callback func(string)) (string, error) {
	l := logger.Get()
	l.Infof("Executing command: %s", command)

	var output string
	err := retry(SSHRetryAttempts, SSHRetryDelay, func() error {
		session, err := c.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		defer session.Close()

		outputBytes, err := session.CombinedOutput(command)
		if err != nil {
			return fmt.Errorf("failed to execute command: %w", err)
		}
		output = string(outputBytes)
		return nil
	})

	return output, err
}

func (c *SSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	return c.PushFileWithCallback(ctx,
		remotePath,
		content,
		executable,
		NullCallback,
	)
}

// PushFile copies a local file to the remote server.
// It takes the local file path and the remote file path as arguments.
// The file is copied over an SSH session using sftp.
// It returns an error if any step of the process fails.

// Refactor the PushFileWithCallback method
func (c *SSHConfig) PushFileWithCallback(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
	_ func(int64, int64),
) error {
	l := logger.Get()
	l.Infof("Pushing file to: %s", remotePath)

	// Create a new SFTP client
	sftpClient, err := sftp.NewClient(c.SSHClient.(*SSHClientWrapper).Client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Ensure the remote directory exists
	dirPath := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(dirPath); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Create or truncate the remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Write the content to the remote file
	if _, err := remoteFile.Write(content); err != nil {
		return fmt.Errorf("failed to write content to remote file: %w", err)
	}

	// Set executable permissions if required
	if executable {
		if err := sftpClient.Chmod(remotePath, 0700); err != nil {
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
	}

	return nil
}

func (c *SSHConfig) InstallSystemdService(
	ctx context.Context,
	serviceName,
	serviceContent string,
) error {
	l := logger.Get()
	l.Infof("Installing systemd service: %s", serviceName)
	remoteServicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	session, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", serviceContent, remoteServicePath)
	err = session.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}

	return nil
}

func (c *SSHConfig) StartService(ctx context.Context, serviceName string) error {
	return c.manageService(ctx, serviceName, "start")
}

func (c *SSHConfig) RestartService(ctx context.Context, serviceName string) error {
	return c.manageService(ctx, serviceName, "restart")
}

func (c *SSHConfig) manageService(_ context.Context, serviceName, action string) error {
	l := logger.Get()
	l.Infof("Managing service: %s %s", serviceName, action)
	session, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("sudo systemctl %s %s", action, serviceName)
	err = session.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to %s service: %w", action, err)
	}

	return nil
}
