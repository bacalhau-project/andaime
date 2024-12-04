package sshutils

import (
	"context"
	"fmt"
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
	SSHClient                 sshutils_interfaces.SSHClienter
	SSHPrivateKeyReader       func(path string) ([]byte, error)
	SSHPublicKeyReader        func(path string) ([]byte, error)
	ClientConfig              *ssh.ClientConfig
	ValidateSSHConnectionFunc func() error
	dialer                    SSHDialer
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
	return &SSHSessionWrapper{Client: w.Client, Session: session}, nil
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
		Timeout: SSHDialTimeout,
		dialer:  &DefaultSSHDialer{},
	}

	// Get SSH client config
	sshClientConfig, err := getSSHClientConfig(user, host, sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH client config: %w", err)
	}
	config.ClientConfig = sshClientConfig

	return config, nil
}

func (c *SSHConfig) createNewConnection() (*ssh.Client, error) {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	if c.ClientConfig == nil {
		config, err := getSSHClientConfig(c.User, c.Host, c.SSHPrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get SSH client config: %w", err)
		}
		c.ClientConfig = config
	}

	if c.dialer == nil {
		c.dialer = &DefaultSSHDialer{}
	}

	client, err := c.dialer.Dial("tcp", addr, c.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return client, nil
}

func (c *SSHConfig) ExecuteCommandWithCallback(
	ctx context.Context,
	command string,
	callback func(string),
) (string, error) {
	// Create a new connection specifically for this command
	client, err := c.createNewConnection()
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
	session.Stdout = &output
	session.Stderr = &output

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
	// Create a new connection specifically for SFTP operations
	client, err := c.createNewConnection()
	if err != nil {
		c.Logger.Errorf("Failed to create SSH connection for file push to %s: %v", remotePath, err)
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Create a new SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		c.Logger.Errorf("Failed to create SFTP client: %v", err)
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(parentDir); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create or overwrite the file
	file, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer file.Close()

	// Write content to file
	n, err := file.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write to remote file: %w", err)
	}
	callback(int64(n), int64(len(content)))

	// Set file permissions if executable
	if executable {
		if err := sftpClient.Chmod(remotePath, 0755); err != nil { //nolint:mnd
			return fmt.Errorf("failed to set executable permissions: %w", err)
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
	newConn, err := c.createNewConnection()
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
			c.Logger.Errorf("Failed to cleanup temporary directory %s: %v (output: %s)", tempDir, err, out)
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
		return fmt.Errorf("failed to move service file to systemd directory: %w (output: %s)", err, out)
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
	startOutput, startErr := c.ExecuteCommand(ctx, fmt.Sprintf("sudo systemctl start %s", serviceName))
	
	// Wait another moment
	time.Sleep(2 * time.Second)
	
	// Check service status
	statusOutput, statusErr := c.ExecuteCommand(ctx, fmt.Sprintf("sudo systemctl status %s", serviceName))
	
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

func (c *SSHConfig) Connect() (sshutils_interfaces.SSHClienter, error) {
	if err := c.ValidateSSHConnectionFunc(); err != nil {
		return nil, fmt.Errorf("SSH connection validation error: %w", err)
	}

	client, err := c.createNewConnection()
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	wrapper := &SSHClientWrapper{Client: client}
	return wrapper, nil
}

func (c *SSHConfig) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	var lastErr error
	deadline := time.Now().Add(timeout)

	// Try immediately first
	if _, err := c.createNewConnection(); err == nil {
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
			if _, err := c.createNewConnection(); err == nil {
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

func (c *SSHConfig) Close() error {
	if c.SSHClient != nil {
		err := c.SSHClient.Close()
		c.SSHClient = nil
		return err
	}
	return nil
}

func (c *SSHConfig) GetHost() string {
	return c.Host
}

func (c *SSHConfig) GetPort() int {
	return c.Port
}

func (c *SSHConfig) GetUser() string {
	return c.User
}

func (c *SSHConfig) GetPrivateKeyMaterial() []byte {
	return c.PrivateKeyMaterial
}

func (c *SSHConfig) SetValidateSSHConnection(fn func() error) {
	c.ValidateSSHConnectionFunc = fn
}

func (c *SSHConfig) GetSSHClient() *ssh.Client {
	if c.SSHClient == nil {
		return nil
	}
	return c.SSHClient.GetClient()
}

func (c *SSHConfig) SetSSHClient(client *ssh.Client) {
	c.SSHClient = &SSHClientWrapper{Client: client}
}

func (c *SSHConfig) SetSSHClienter(client sshutils_interfaces.SSHClienter) {
	c.SSHClient = client
}

func (c *SSHConfig) ExecuteCommand(ctx context.Context, command string) (string, error) {
	return c.ExecuteCommandWithCallback(ctx, command, func(output string) {})
}

func (c *SSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	return c.PushFileWithCallback(ctx, remotePath, content, executable, func(int64, int64) {})
}

func getSSHClientConfig(user, host, privateKeyPath string) (*ssh.ClientConfig, error) {
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
