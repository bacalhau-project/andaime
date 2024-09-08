package sshutils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"golang.org/x/crypto/ssh"
)

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
	PushFile(ctx context.Context, remotePath string, content []byte, executable bool) error
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

	// TODO: Handle host key callback
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	possibleHostKeyCallback, err := GetHostKeyCallback(host)
	if err != nil {
		hostKeyCallback = possibleHostKeyCallback
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}, nil
}

func getPrivateKey(privateKeyMaterial string) (ssh.Signer, error) {
	privateKey, err := ssh.ParsePrivateKey([]byte(privateKeyMaterial))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return privateKey, nil
}

func (c *SSHConfig) SetSSHClient(client SSHClienter) {
	c.SSHClient = client
}

func (c *SSHConfig) Connect() (SSHClienter, error) {
	l := logger.Get()
	l.Infof("Connecting to SSH server: %s:%d", c.Host, c.Port)

	client, err := c.SSHDial.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), c.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
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

	return c.SSHClient.NewSession()
}

// ExecuteCommand runs a command on the remote server over SSH.
// It takes the command as a string argument.
// It retries the execution a configurable number of times if it fails.
// It returns the output of the command as a string and any error encountered.
func (c *SSHConfig) ExecuteCommand(ctx context.Context, command string) (string, error) {
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

// PushFile copies a local file to the remote server.
// It takes the local file path and the remote file path as arguments.
// The file is copied over an SSH session using the stdin pipe.
// It returns an error if any step of the process fails.
func (c *SSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	l := logger.Get()
	l.Infof("Pushing file to: %s", remotePath)

	session, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	remoteCmd := fmt.Sprintf("cat > %s", remotePath)
	if executable {
		remoteCmd += " && chmod +x " + remotePath
	}

	// Create a pipe for the session's stdin
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	defer stdin.Close()

	// Start the remote command
	err = session.Start(remoteCmd)
	if err != nil {
		return fmt.Errorf("failed to start remote command: %w", err)
	}

	// Copy the content to the remote command's stdin
	_, err = stdin.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write file contents: %w", err)
	}

	// Close the stdin pipe to signal end of input
	err = stdin.Close()
	if err != nil {
		return fmt.Errorf("failed to close stdin pipe: %w", err)
	}

	// Wait for the remote command to finish
	err = session.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for remote command: %w", err)
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
