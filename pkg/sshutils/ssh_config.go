package sshutils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the configuration for SSH connections
type SSHConfig struct {
	Host                      string
	Port                      int
	User                      string
	SSHPrivateKeyPath         string
	PrivateKeyMaterial        []byte
	Timeout                   time.Duration
	Logger                    *logger.Logger
	SSHClient                 *ssh.Client
	SSHPrivateKeyReader       func(path string) ([]byte, error)
	SSHPublicKeyReader        func(path string) ([]byte, error)
	clientCreator             sshutils_interfaces.SSHClientCreator
	sftpClientCreator         sshutils_interfaces.SFTPClientCreator
	ClientConfig              *ssh.ClientConfig
	ValidateSSHConnectionFunc func() error
}

// SSHDialer is an interface for creating SSH connections
type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

// DefaultSSHDialer implements SSHDialer using the standard ssh.Dial
type DefaultSSHDialer struct{}

func (d *DefaultSSHDialer) Dial(
	network, addr string,
	config *ssh.ClientConfig,
) (*ssh.Client, error) {
	return ssh.Dial(network, addr, config)
}

// NewSSHConfigFunc is the function used to create new SSH configurations
// This can be overridden for testing
var NewSSHConfigFunc = NewSSHConfig

func (c *SSHConfig) ExecuteCommandWithCallback(
	ctx context.Context,
	command string,
	callback func(string),
) (string, error) {
	// Create a new connection specifically for this command
	client, err := c.clientCreator.NewClient(
		c.Host,
		c.Port,
		c.User,
		c.SSHPrivateKeyPath,
		c.ClientConfig,
	)
	if err != nil {
		c.Logger.Errorf("Failed to create SSH connection for command %s: %v", command, err)
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		c.Logger.Errorf("Failed to create session for command %s: %v", command, err)
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var output strings.Builder

	session.SetStdout(&output)
	session.SetStderr(&output)

	if err := session.Run(command); err != nil {
		c.Logger.Errorf("Failed to run command %s: %v", command, err)
		return output.String(), fmt.Errorf("command execution failed: %w", err)
	}

	outputStr := output.String()
	callback(outputStr)
	return outputStr, nil
}

func (c *SSHConfig) PushFileWithCallback(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
	callback func(int64, int64),
) error {
	// Use existing client if available, otherwise create new one
	var sshClient sshutils_interfaces.SSHClienter
	var err error

	if c.SSHClient != nil {
		sshClient = NewSSHClientWrapper(c.SSHClient)
	} else {
		sshClient, err = c.clientCreator.NewClient(
			c.Host,
			c.Port,
			c.User,
			c.SSHPrivateKeyPath,
			c.ClientConfig,
		)
		if err != nil {
			c.Logger.Errorf("Failed to create SSH connection for file push to %s: %v", remotePath, err)
			return fmt.Errorf("SSH connection failed: %w", err)
		}
		defer sshClient.Close()
	}

	// Use sftpClientCreator if set, otherwise fall back to default
	if c.sftpClientCreator == nil {
		c.sftpClientCreator = &defaultSFTPClientCreator{}
	}

	// Create SFTP client
	sftpClient, err := c.GetSFTPClientCreator().NewSFTPClient(sshClient)
	if err != nil {
		c.Logger.Errorf("Failed to create SFTP client: %v", err)
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(parentDir); err != nil {
		c.Logger.Errorf("Failed to create parent directory %s: %v", parentDir, err)
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create or overwrite the file
	file, err := sftpClient.Create(remotePath)
	if err != nil {
		c.Logger.Errorf("Failed to create remote file %s: %v", remotePath, err)
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer file.Close()

	// Write content to file
	n, err := file.Write(content)
	if err != nil {
		c.Logger.Errorf("Failed to write content to %s: %v", remotePath, err)
		return fmt.Errorf("failed to write to remote file: %w", err)
	}
	callback(int64(n), int64(len(content)))

	// Set file permissions if executable
	if executable {
		if err := sftpClient.Chmod(remotePath, 0755); err != nil { //nolint:mnd
			c.Logger.Errorf("Failed to set executable permissions on %s: %v", remotePath, err)
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
	}

	// Verify file exists and is executable
	fileInfo, err := sftpClient.Stat(remotePath)
	if err != nil {
		c.Logger.Errorf("Failed to stat file %s after transfer: %v", remotePath, err)
		return fmt.Errorf("failed to verify file after transfer: %w", err)
	}

	c.Logger.Debugf("Successfully pushed file to %s (size: %d bytes, executable: %v, mode: %v)",
		remotePath, len(content), executable, fileInfo.Mode())

	// Additional verification: list directory contents
	parentDir = filepath.Dir(remotePath)
	files, err := sftpClient.ReadDir(parentDir)
	if err != nil {
		c.Logger.Errorf("Failed to list directory contents for %s: %v", parentDir, err)
	} else {
		c.Logger.Debugf("Files in %s:", parentDir)
		for _, f := range files {
			c.Logger.Debugf("- %s (mode: %v)", f.Name(), f.Mode())
		}
	}

	return nil
}

func (c *SSHConfig) InstallSystemdService(
	ctx context.Context,
	serviceName string,
	serviceContent string,
) error {
	// Create SFTP client
	newConn, err := c.clientCreator.NewClient(
		c.Host,
		c.Port,
		c.User,
		c.SSHPrivateKeyPath,
		c.ClientConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH connection: %w", err)
	}
	sftpClient, err := currentSFTPClientCreator(newConn)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create temporary directory with timestamp
	timestamp := time.Now().Unix()
	tempDir := fmt.Sprintf("/tmp/andaime-%d", timestamp)
	if err := sftpClient.MkdirAll(tempDir); err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if out, err := c.ExecuteCommand(ctx, fmt.Sprintf("rm -rf %s", tempDir)); err != nil {
			c.Logger.Errorf(
				"Failed to cleanup temporary directory %s: %v (output: %s)",
				tempDir,
				err,
				out,
			)
		}
	}()

	// Create temporary service file
	tempServicePath := fmt.Sprintf("%s/%s.service", tempDir, serviceName)
	file, err := sftpClient.Create(tempServicePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary service file: %w", err)
	}
	defer file.Close()

	// Write service content to temporary file
	if _, err := file.Write([]byte(serviceContent)); err != nil {
		return fmt.Errorf("failed to write service content: %w", err)
	}
	file.Close()

	// Move file to systemd directory with sudo
	systemdPath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	moveCmd := fmt.Sprintf("sudo mv %s %s", tempServicePath, systemdPath)
	if out, err := c.ExecuteCommand(ctx, moveCmd); err != nil {
		return fmt.Errorf(
			"failed to move service file to systemd directory: %w (output: %s)",
			err,
			out,
		)
	}

	// Set correct permissions with sudo
	chmodCmd := fmt.Sprintf("sudo chmod 644 %s", systemdPath)
	if out, err := c.ExecuteCommand(ctx, chmodCmd); err != nil {
		return fmt.Errorf("failed to set service file permissions: %w (output: %s)", err, out)
	}

	// Reload systemd daemon with sudo
	if out, err := c.ExecuteCommand(ctx, "sudo systemctl daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w (output: %s)", err, out)
	} else {
		c.Logger.Infof("systemctl daemon-reload output: %s", out)
	}

	// Enable service with sudo
	if out, err := c.ExecuteCommand(ctx, fmt.Sprintf("sudo systemctl enable %s", serviceName)); err != nil {
		return fmt.Errorf("failed to enable service: %w (output: %s)", err, out)
	} else {
		c.Logger.Infof("systemctl enable %s output: %s", serviceName, out)
	}

	return nil
}

func (c *SSHConfig) StartService(ctx context.Context, serviceName string) (string, error) {
	output, err := c.ExecuteCommand(ctx, fmt.Sprintf("sudo systemctl start %s", serviceName))
	return output, err
}

func (c *SSHConfig) RestartService(ctx context.Context, serviceName string) (string, error) {
	// Attempt to stop the service
	stopOutput, stopErr := c.ExecuteCommand(ctx, fmt.Sprintf("sudo systemctl stop %s", serviceName))

	// Wait a short moment
	time.Sleep(2 * time.Second)

	// Attempt to start the service
	startOutput, startErr := c.ExecuteCommand(
		ctx,
		fmt.Sprintf("sudo systemctl start %s", serviceName),
	)

	// Wait another moment
	time.Sleep(2 * time.Second)

	// Check service status
	statusOutput, statusErr := c.ExecuteCommand(
		ctx,
		fmt.Sprintf("sudo systemctl status %s", serviceName),
	)

	// Combine all outputs for logging
	combinedOutput := fmt.Sprintf("Stop Output: %s\nStart Output: %s\nStatus Output: %s",
		stopOutput, startOutput, statusOutput)

	// Determine the final error
	var finalErr error
	if stopErr != nil {
		finalErr = fmt.Errorf("stop service failed: %w", stopErr)
	}
	if startErr != nil {
		if finalErr != nil {
			finalErr = fmt.Errorf("%v; start service failed: %w", finalErr, startErr)
		} else {
			finalErr = fmt.Errorf("start service failed: %w", startErr)
		}
	}
	if statusErr != nil {
		if finalErr != nil {
			finalErr = fmt.Errorf("%v; status check failed: %w", finalErr, statusErr)
		} else {
			finalErr = fmt.Errorf("status check failed: %w", statusErr)
		}
	}

	// Log the full details for debugging
	if finalErr != nil {
		c.Logger.Errorf("Service restart failed for %s. Details: %s. Error: %v",
			serviceName, combinedOutput, finalErr)
	}

	return combinedOutput, finalErr
}

// NewSSHConfig creates a new SSH configuration
func NewSSHConfig(
	host string,
	port int,
	user string,
	privateKeyPath string,
) (sshutils_interfaces.SSHConfiger, error) {
	config := &SSHConfig{
		Host:              host,
		Port:              port,
		User:              user,
		SSHPrivateKeyPath: privateKeyPath,
		Logger:            logger.Get(),
		clientCreator:     DefaultSSHClientCreatorInstance,
		sftpClientCreator: DefaultSFTPClientCreator,
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
			if privateKeyPath == "" {
				return fmt.Errorf("private key path is empty")
			}
			return nil
		},
		Timeout: SSHDialTimeout,
	}

	clientConfig, err := getSSHClientConfig(user, privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH client config: %w", err)
	}
	config.ClientConfig = clientConfig

	return config, nil
}

// Connect establishes an SSH connection
func (c *SSHConfig) Connect() (sshutils_interfaces.SSHClienter, error) {
	// If we already have a client, return a wrapper around it
	if c.SSHClient != nil {
		return NewSSHClientWrapper(c.SSHClient), nil
	}

	// Create new client using the client creator
	client, err := c.clientCreator.NewClient(
		c.Host,
		c.Port,
		c.User,
		c.SSHPrivateKeyPath,
		c.ClientConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH client: %w", err)
	}

	// Store the underlying SSH client
	if sshClient, ok := client.(*SSHClientWrapper); ok {
		c.SSHClient = sshClient.Client
	}

	return client, nil
}

// SetClientCreator allows injection of a custom client creator (useful for testing)
func (c *SSHConfig) SetSSHClientCreator(creator sshutils_interfaces.SSHClientCreator) {
	c.clientCreator = creator
}

func (c *SSHConfig) ExecuteCommand(ctx context.Context, command string) (string, error) {
	// Create a new connection specifically for this command
	client, err := c.clientCreator.NewClient(
		c.Host,
		c.Port,
		c.User,
		c.SSHPrivateKeyPath,
		c.ClientConfig,
	)
	if err != nil {
		c.Logger.Errorf("Failed to create SSH connection for command %s: %v", command, err)
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		c.Logger.Errorf("Failed to create session for command %s: %v", command, err)
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var output strings.Builder
	session.SetStdout(&output)
	session.SetStderr(&output)

	if err := session.Run(command); err != nil {
		c.Logger.Errorf("Failed to run command %s: %v", command, err)
		return output.String(), fmt.Errorf("command execution failed: %w", err)
	}

	return output.String(), nil
}

func (c *SSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	return c.PushFileWithCallback(ctx, remotePath, content, executable, func(int64, int64) {})
}

func getSSHClientConfig(user, privateKeyPath string) (*ssh.ClientConfig, error) {
	if privateKeyPath == "" {
		return nil, fmt.Errorf("private key path is empty")
	}

	// Expand the path to handle ~
	expandedPath := os.ExpandEnv(privateKeyPath)
	if strings.HasPrefix(expandedPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		expandedPath = filepath.Join(home, expandedPath[2:])
	}

	privateKeyBytes, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key from %s: %w", privateKeyPath, err)
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
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

func currentSFTPClientCreator(client sshutils_interfaces.SSHClienter) (*sftp.Client, error) {
	sshClient := client.GetClient()
	if sshClient == nil {
		return nil, fmt.Errorf("invalid SSH client")
	}
	return sftp.NewClient(sshClient)
}

// DefaultSSHClientCreator implements SSHClientCreator interface
type DefaultSSHClientCreator struct{}

func (d *DefaultSSHClientCreator) NewClient(
	host string,
	port int,
	user string,
	privateKeyPath string,
	config *ssh.ClientConfig,
) (sshutils_interfaces.SSHClienter, error) {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return NewSSHClientWrapper(client), nil
}

// DefaultSSHClientCreatorInstance is the default SSH client creator instance
var DefaultSSHClientCreatorInstance = &DefaultSSHClientCreator{}

// Close closes the SSH client connection
func (c *SSHConfig) Close() error {
	if c.SSHClient != nil {
		err := c.SSHClient.Close()
		c.SSHClient = nil
		return err
	}
	return nil
}

// GetHost returns the SSH host
func (c *SSHConfig) GetHost() string {
	return c.Host
}

// GetPort returns the SSH port
func (c *SSHConfig) GetPort() int {
	return c.Port
}

// GetUser returns the SSH user
func (c *SSHConfig) GetUser() string {
	return c.User
}

// GetPrivateKeyMaterial returns the SSH private key material
func (c *SSHConfig) GetPrivateKeyMaterial() []byte {
	return c.PrivateKeyMaterial
}

func (c *SSHConfig) GetSSHPrivateKeyPath() string {
	return c.SSHPrivateKeyPath
}

// SetValidateSSHConnection sets the SSH connection validation function
func (c *SSHConfig) SetValidateSSHConnection(fn func() error) {
	c.ValidateSSHConnectionFunc = fn
}

// GetSSHClient returns the underlying SSH client
func (c *SSHConfig) GetSSHClient() *ssh.Client {
	if c.SSHClient == nil {
		return nil
	}
	return c.SSHClient
}

// SetSSHClient sets the SSH client
func (c *SSHConfig) SetSSHClient(client *ssh.Client) {
	if client == nil {
		c.SSHClient = nil
	} else {
		c.SSHClient = client
	}
}

// WaitForSSH waits for SSH connection to be available
func (c *SSHConfig) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	var lastErr error
	deadline := time.Now().Add(timeout)

	// Try immediately first
	if _, err := c.clientCreator.NewClient(
		c.Host,
		c.Port,
		c.User,
		c.SSHPrivateKeyPath,
		c.ClientConfig,
	); err == nil {
		return nil
	} else {
		lastErr = err
		fmt.Printf("Could not connect: %v\nRetrying in %v seconds...\n", err, SSHRetryDelay.Seconds())
	}

	// If first attempt fails, start retrying with backoff
	ticker := time.NewTicker(SSHRetryDelay)
	defer ticker.Stop()

	attempts := 1

	for attempts < retry && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for SSH: %w", lastErr)
		case <-ticker.C:
			// Try to connect
			if _, err := c.clientCreator.NewClient(
				c.Host,
				c.Port,
				c.User,
				c.SSHPrivateKeyPath,
				c.ClientConfig,
			); err == nil {
				return nil
			} else {
				lastErr = err
				attempts++
				fmt.Printf("Could not connect: %v\nRetrying in %v seconds... (attempt %d/%d)\n",
					err, SSHRetryDelay.Seconds(), attempts, retry)
			}
		}
	}

	if attempts >= retry {
		return fmt.Errorf("max retries (%d) exceeded: %w", retry, lastErr)
	}

	if time.Now().After(deadline) {
		return fmt.Errorf("timeout after %v: %w", timeout, lastErr)
	}

	return lastErr
}

// GetSSHClientCreator returns the SSH client creator
func (c *SSHConfig) GetSSHClientCreator() sshutils_interfaces.SSHClientCreator {
	return c.clientCreator
}

// Add IsConnected method
func (c *SSHConfig) IsConnected() bool {
	return c.SSHClient != nil
}

// Add these methods after the existing methods
func (c *SSHConfig) SetSFTPClientCreator(creator sshutils_interfaces.SFTPClientCreator) {
	c.sftpClientCreator = creator
}

func (c *SSHConfig) GetSFTPClientCreator() sshutils_interfaces.SFTPClientCreator {
	return c.sftpClientCreator
}

// SSHSessionWrapper implements SSHSessioner interface
type SSHSessionWrapper struct {
	Session *ssh.Session
}

func (w *SSHSessionWrapper) Run(cmd string) error {
	return w.Session.Run(cmd)
}

func (w *SSHSessionWrapper) Close() error {
	return w.Session.Close()
}

func (w *SSHSessionWrapper) SetStdout(writer io.Writer) {
	w.Session.Stdout = writer
}

func (w *SSHSessionWrapper) SetStderr(writer io.Writer) {
	w.Session.Stderr = writer
}

func (w *SSHSessionWrapper) CombinedOutput(cmd string) ([]byte, error) {
	return w.Session.CombinedOutput(cmd)
}

func (w *SSHSessionWrapper) StdinPipe() (io.WriteCloser, error) {
	return w.Session.StdinPipe()
}

func (w *SSHSessionWrapper) Start(cmd string) error {
	return w.Session.Start(cmd)
}

func (w *SSHSessionWrapper) Wait() error {
	return w.Session.Wait()
}

func (w *SSHSessionWrapper) Signal(sig ssh.Signal) error {
	return w.Session.Signal(sig)
}

// SSHClientWrapper implements SSHClienter interface
type SSHClientWrapper struct {
	Client *ssh.Client
}

func (w *SSHClientWrapper) NewSession() (sshutils_interfaces.SSHSessioner, error) {
	if w.Client == nil {
		return nil, fmt.Errorf("SSH client is not connected")
	}
	session, err := w.Client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	return &SSHSessionWrapper{Session: session}, nil
}

func (w *SSHClientWrapper) Close() error {
	if w.Client != nil {
		return w.Client.Close()
	}
	return nil
}

func (w *SSHClientWrapper) GetClient() *ssh.Client {
	return w.Client
}

func (w *SSHClientWrapper) Connect() (sshutils_interfaces.SSHClienter, error) {
	return w, nil
}

func (w *SSHClientWrapper) IsConnected() bool {
	return w.Client != nil
}

// NewSSHClientWrapper creates a new SSHClientWrapper
func NewSSHClientWrapper(client *ssh.Client) *SSHClientWrapper {
	return &SSHClientWrapper{Client: client}
}
