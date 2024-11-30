package sshutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
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
	SSHClient                 SSHClienter
	SSHDial                   SSHDialer
	SSHPrivateKeyReader       func(path string) ([]byte, error)
	SSHPublicKeyReader        func(path string) ([]byte, error)
	ClientConfig              *ssh.ClientConfig
	ValidateSSHConnectionFunc func() error
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
) (SSHConfiger, error) {
	l := logger.Get()
	l.Debugf("Creating new SSH config for %s@%s:%d", user, host, port)

	config := &SSHConfig{
		Host:                host,
		Port:                port,
		User:                user,
		SSHPrivateKeyPath:   sshPrivateKeyPath,
		Logger:              l,
		SSHDial:             &defaultSSHDialer{},
		SSHPrivateKeyReader: ioutil.ReadFile,
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

func (c *SSHConfig) Connect() (SSHClienter, error) {
	l := logger.Get()
	l.Infof("Connecting to SSH server: %s:%d", c.Host, c.Port)

	// Validate connection prerequisites
	if err := c.ValidateSSHConnectionFunc(); err != nil {
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

		if i < SSHRetryAttempts-1 {
			l.Debugf("Failed to connect, retrying in %v: %v\n", SSHRetryDelay, err)
			time.Sleep(SSHRetryDelay)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect after %d attempts: %w", SSHRetryAttempts, err)
	}

	c.SSHClient = client
	return client, nil
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

// GetSSHDial returns the SSH dialer
func (c *SSHConfig) GetSSHDial() SSHDialer {
	return c.SSHDial
}

// SetSSHDial sets the SSH dialer
func (c *SSHConfig) SetSSHDial(dialer SSHDialer) {
	c.SSHDial = dialer
}

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
func (c *SSHConfig) SetSSHClienter(client SSHClienter) {
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
	session, err := c.SSHClient.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	return string(output), err
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
	session, err := c.SSHClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	fileMode := "644"
	if executable {
		fileMode = "755"
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("cat > %s && chmod %s %s", remotePath, mode, remotePath)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start file push: %w", err)
	}

	if _, err := stdin.Write(content); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	if err := stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to push file: %w", err)
	}

	return nil
}

// PushFileWithCallback pushes a file with progress callback
func (c *SSHConfig) PushFileWithCallback(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
	callback func(int64, int64),
) error {
	session, err := c.SSHClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	totalSize := int64(len(content))
	var written int64

	for written < totalSize {
		chunkSize := int64(4096)
		if written+chunkSize > totalSize {
			chunkSize = totalSize - written
		}

		fileMode := "644"
		if executable {
			fileMode = "755"
		}
		cmd := fmt.Sprintf("cat > %s", remotePath)
		
		stdin, err := session.StdinPipe()
		if err != nil {
			return err
		}

		err = session.Start(cmd)
		if err != nil {
			return err
		}

		_, err = stdin.Write(content[written : written+chunkSize])
		if err != nil {
			return err
		}

		written += chunkSize
		callback(written, totalSize)

		err = stdin.Close()
		if err != nil {
			return err
		}

		err = session.Wait()
		if err != nil {
			return err
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
	tmpFile, err := ioutil.TempFile("", "systemd-service-")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(serviceContent)); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := c.PushFile(ctx, filepath.Join("/etc/systemd/system", serviceName), []byte(serviceContent), false); err != nil {
		return err
	}

	if err := c.ExecuteCommand(ctx, fmt.Sprintf("systemctl daemon-reload")); err != nil {
		return err
	}

	if err := c.ExecuteCommand(ctx, fmt.Sprintf("systemctl enable %s", serviceName)); err != nil {
		return err
	}

	return nil
}

func (c *SSHConfig) StartService(ctx context.Context, serviceName string) error {
	return c.ExecuteCommand(ctx, fmt.Sprintf("systemctl start %s", serviceName))
}

func (c *SSHConfig) RestartService(ctx context.Context, serviceName string) error {
	return c.ExecuteCommand(ctx, fmt.Sprintf("systemctl restart %s", serviceName))
}

// Removed type declarations

type defaultSSHDialer struct{}

func (d *defaultSSHDialer) Dial(
	network, addr string,
	config *ssh.ClientConfig,
) (SSHClienter, error) {
	client, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	return &SSHClientWrapper{Client: client}, nil
}

func (d *defaultSSHDialer) DialContext(
	ctx context.Context,
	network, addr string,
	config *ssh.ClientConfig,
) (SSHClienter, error) {
	type dialResult struct {
		client SSHClienter
		err    error
	}

	result := make(chan dialResult, 1)

	go func() {
		client, err := d.Dial(network, addr, config)
		result <- dialResult{client, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-result:
		return res.client, res.err
	}
}

func getSSHClientConfig(user, host, privateKeyPath string) (*ssh.ClientConfig, error) {
	l := logger.Get()
	l.Debugf("Getting SSH client config for %s", host)

	if privateKeyPath == "" {
		return nil, fmt.Errorf("private key path is empty")
	}

	privateKeyBytes, err := ioutil.ReadFile(privateKeyPath)
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
