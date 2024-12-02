package sshutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the configuration for SSH connections
type SSHConfig struct {
	Host               string
	Port               int
	User               string
	SSHPrivateKeyPath  string
	PrivateKeyMaterial []byte
	Timeout            time.Duration
	Logger             *logger.Logger
	SSHClient          sshutils_interfaces.SSHClienter
	// SSHDial has been removed
	SSHPrivateKeyReader       func(path string) ([]byte, error)
	SSHPublicKeyReader        func(path string) ([]byte, error)
	ClientConfig              *ssh.ClientConfig
	ValidateSSHConnectionFunc func() error
}

// SSHClientWrapper implements SSHClienter interface
type SSHClientWrapper struct {
	Client *ssh.Client
}

func (w *SSHClientWrapper) NewSession() (sshutils_interfaces.SSHSessioner, error) {
	session, err := w.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &SSHSessionWrapper{Session: session}, nil
}

func (w *SSHClientWrapper) Close() error {
	return w.Client.Close()
}

func (w *SSHClientWrapper) GetClient() *ssh.Client {
	return w.Client
}

// NewSSHConfigFunc is the function used to create new SSH configurations
// This can be overridden for testing
var NewSSHConfigFunc = NewSSHConfig

// NewSSHConfig creates a new SSH configuration
func NewSSHConfig(
	host string,
	port int,
	user string,
	sshPrivateKeyPath string,
) (sshutils_interfaces.SSHConfiger, error) {
	l := logger.Get()
	l.Debugf("Creating new SSH config for %s@%s:%d", user, host, port)

	config := &SSHConfig{
		Host:              host,
		Port:              port,
		User:              user,
		SSHPrivateKeyPath: sshPrivateKeyPath,
		Logger:            l,
		// SSHDial has been removed
		ValidateSSHConnectionFunc: func() error {
			if host == "" {
				return fmt.Errorf("host cannot be empty")
			}
			if port <= 0 {
				return fmt.Errorf("invalid port number: %d", port)
			}
			if user == "" {
				return fmt.Errorf("user cannot be empty")
			}
			if sshPrivateKeyPath == "" {
				return fmt.Errorf("SSH private key path cannot be empty")
			}
			return nil
		},
	}

	// Get SSH client config
	sshClientConfig, err := getSSHClientConfig(user, host, sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH client config: %w", err)
	}
	config.ClientConfig = sshClientConfig

	return config, nil
}

var DialSSHFunc = dialSSH

func (c *SSHConfig) Connect() (sshutils_interfaces.SSHClienter, error) {
	l := logger.Get()
	l.Infof("🔐 Attempting SSH Connection: %s@%s:%d", c.User, c.Host, c.Port)

	// Extensive connection validation
	if err := c.ValidateSSHConnectionFunc(); err != nil {
		l.Errorf("❌ SSH Connection Validation Failed: %v", err)
		return nil, fmt.Errorf("SSH connection validation error: %w", err)
	}

	// Log detailed connection parameters
	l.Debugf("🔍 Connection Parameters:\n" +
		"  Host: %s\n" +
		"  Port: %d\n" +
		"  User: %s\n" +
		"  Private Key Path: %s\n" +
		"  Timeout: %v\n" +
		"  Retry Attempts: %d",
		c.Host, c.Port, c.User, c.SSHPrivateKeyPath, 
		c.Timeout, SSHRetryAttempts)

	var err error
	var client sshutils_interfaces.SSHClienter

	for attempt := 1; attempt <= SSHRetryAttempts; attempt++ {
		l.Debugf("🔄 Connection Attempt %d/%d", attempt, SSHRetryAttempts)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()

		client, err = dialSSHContext(
			ctx,
			"tcp",
			fmt.Sprintf("%s:%d", c.Host, c.Port),
			c.ClientConfig,
		)

		if err == nil {
			l.Infof("✅ Successfully established SSH connection to %s:%d", c.Host, c.Port)
			c.SSHClient = client
			return client, nil
		}

		// Log specific error details
		l.Errorf("❌ SSH Connection Attempt %d Failed: %v", attempt, err)
		
		// Detailed error type logging
		switch {
		case os.IsTimeout(err):
			l.Errorf("🕒 Connection Timeout: Network or SSH service might be unresponsive")
		case strings.Contains(err.Error(), "connection refused"):
			l.Errorf("🚫 Connection Refused: Check firewall, SSH service status")
		case strings.Contains(err.Error(), "no route to host"):
			l.Errorf("🌐 Network Unreachable: Check network connectivity")
		case strings.Contains(err.Error(), "invalid key"):
			l.Errorf("🔑 SSH Key Authentication Failed: Verify private key")
		}

		if attempt < SSHRetryAttempts {
			backoffTime := time.Duration(attempt*2) * time.Second
			l.Debugf("⏳ Waiting %v before next attempt", backoffTime)
			time.Sleep(backoffTime)
		}
	}

	finalErr := fmt.Errorf("❌ SSH Connection Failed after %d attempts: %w", 
		SSHRetryAttempts, err)
	l.Errorf(finalErr.Error())
	return nil, finalErr
}

func (c *SSHConfig) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	l := logger.Get()
	l.Debugf("Waiting for SSH connection to %s:%d", c.Host, c.Port)

	for i := 0; i < retry; i++ {
		client, err := c.Connect()
		if err == nil {
			defer client.Close()
			return nil
		}

		l.Debugf("Failed to connect to SSH server (attempt %d/%d): %v", i+1, retry, err)
		if i < retry-1 {
			time.Sleep(timeout)
		}
	}

	return fmt.Errorf("failed to connect to SSH server after %d attempts", retry)
}

// GetHost returns the configured host
func (c *SSHConfig) GetHost() string {
	return c.Host
}

// GetPort returns the configured port
func (c *SSHConfig) GetPort() int {
	return c.Port
}

// GetUser returns the configured user
func (c *SSHConfig) GetUser() string {
	return c.User
}

// GetPrivateKeyMaterial returns the private key material
func (c *SSHConfig) GetPrivateKeyMaterial() []byte {
	return c.PrivateKeyMaterial
}

// SSHDial methods removed to break import cycle

// SetValidateSSHConnection sets the validation function
func (c *SSHConfig) SetValidateSSHConnection(fn func() error) {
	c.ValidateSSHConnectionFunc = fn
}

// GetSSHClient returns the underlying SSH client
func (c *SSHConfig) GetSSHClient() *ssh.Client {
	if wrapper, ok := c.SSHClient.(*SSHClientWrapper); ok {
		return wrapper.Client
	}
	return nil
}

// SetSSHClient sets the underlying SSH client
func (c *SSHConfig) SetSSHClient(client *ssh.Client) {
	c.SSHClient = &SSHClientWrapper{Client: client}
}

// SetSSHClienter sets the SSH client
func (c *SSHConfig) SetSSHClienter(client sshutils_interfaces.SSHClienter) {
	c.SSHClient = client
}

// Close closes the SSH connection
func (c *SSHConfig) Close() error {
	if c.SSHClient != nil {
		return c.SSHClient.Close()
	}
	return nil
}

// ExecuteCommand executes a command over SSH and returns its output
func (c *SSHConfig) ExecuteCommand(ctx context.Context, command string) (string, error) {
	return c.ExecuteCommandWithCallback(ctx, command, func(output string) {})
}

// ExecuteCommandWithCallback executes a command over SSH with output callback
func (c *SSHConfig) ExecuteCommandWithCallback(
	ctx context.Context,
	command string,
	callback func(string),
) (string, error) {
	session, err := c.SSHClient.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", err
	}

	callback(string(output))
	return string(output), nil
}

// PushFile pushes a file to the remote host
func (c *SSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	return c.PushFileWithCallback(ctx, remotePath, content, executable, func(int64, int64) {})
}

// PushFileWithCallback pushes a file with progress callback
func (c *SSHConfig) PushFileWithCallback(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
	callback func(int64, int64),
) error {
	// Get SFTP client
	sftpClient, err := DefaultSFTPClientCreator(c.SSHClient.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	totalSize := int64(len(content))
	var written int64

	// Write file contents in chunks
	for written < totalSize {
		chunkSize := int64(4096) //nolint:mnd
		if written+chunkSize > totalSize {
			chunkSize = totalSize - written
		}

		n, err := remoteFile.Write(content[written : written+chunkSize])
		if err != nil {
			return fmt.Errorf("failed to write to remote file: %w", err)
		}

		written += int64(n)
		callback(written, totalSize)
	}

	// Set file permissions if executable
	if executable {
		if err := sftpClient.Chmod(remotePath, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
	}

	return nil
}

// InstallSystemdService installs a systemd service
func (c *SSHConfig) InstallSystemdService(
	ctx context.Context,
	serviceName string,
	serviceContent string,
) error {
	// Create SFTP client
	sftpClient, err := DefaultSFTPClientCreator(c.SSHClient.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Construct remote path
	remotePath := filepath.Join("/etc/systemd/system", serviceName)

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote systemd service file: %w", err)
	}
	defer remoteFile.Close()

	// Write service content
	if _, err := remoteFile.Write([]byte(serviceContent)); err != nil {
		return fmt.Errorf("failed to write service content: %w", err)
	}

	// Reload systemd daemon
	if out, err := c.ExecuteCommand(ctx, "systemctl daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	} else {
		logger.Get().Infof("systemctl daemon-reload output: %s", out)
	}

	// Enable service
	if out, err := c.ExecuteCommand(ctx, fmt.Sprintf("systemctl enable %s", serviceName)); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	} else {
		logger.Get().Infof("systemctl enable %s output: %s", serviceName, out)
	}

	return nil
}

func (c *SSHConfig) StartService(ctx context.Context, serviceName string) (string, error) {
	output, err := c.ExecuteCommand(ctx, fmt.Sprintf("systemctl start %s", serviceName))
	return output, err
}

func (c *SSHConfig) RestartService(ctx context.Context, serviceName string) (string, error) {
	output, err := c.ExecuteCommand(ctx, fmt.Sprintf("systemctl restart %s", serviceName))
	return output, err
}

func dialSSH(
	network, addr string,
	config *ssh.ClientConfig,
) (sshutils_interfaces.SSHClienter, error) {
	client, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	return &SSHClientWrapper{Client: client}, nil
}

func dialSSHContext(
	ctx context.Context,
	network, addr string,
	config *ssh.ClientConfig,
) (sshutils_interfaces.SSHClienter, error) {
	l := logger.Get()
	l.Debugf("🌐 Initiating SSH Dial Context: %s, %s", network, addr)

	type dialResult struct {
		client sshutils_interfaces.SSHClienter
		err    error
	}

	result := make(chan dialResult, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		client, err := dialSSH(network, addr, config)
		result <- dialResult{client, err}
	}()

	select {
	case <-ctx.Done():
		l.Errorf("❌ SSH Dial Context Cancelled: %v", ctx.Err())
		return nil, fmt.Errorf("SSH dial context cancelled: %w", ctx.Err())
	case res := <-result:
		if res.err != nil {
			l.Errorf("❌ SSH Dial Failed: %v", res.err)
			return nil, fmt.Errorf("SSH dial error: %w", res.err)
		}
		l.Debugf("✅ SSH Dial Successful")
		return res.client, nil
	case <-time.After(config.Timeout):
		l.Errorf("⏰ SSH Dial Timeout after %v", config.Timeout)
		return nil, fmt.Errorf("SSH dial timeout after %v", config.Timeout)
	}
}

func getSSHClientConfig(user, host, privateKeyPath string) (*ssh.ClientConfig, error) {
	l := logger.Get()
	l.Debugf("Getting SSH client config for %s", host)

	if privateKeyPath == "" {
		return nil, fmt.Errorf("private key path is empty")
	}

	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := getPrivateKey(string(privateKeyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         SSHDialTimeout,
	}

	return config, nil
}

func getPrivateKey(privateKeyMaterial string) (ssh.Signer, error) {
	privateKey, err := ssh.ParsePrivateKey([]byte(privateKeyMaterial))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return privateKey, nil
}

func currentSFTPClientCreator(client *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(client)
}
