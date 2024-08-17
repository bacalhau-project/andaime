package sshutils

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	Host                  string
	Port                  int
	User                  string
	PublicKeyPath         string
	PrivateKeyPath        string
	Timeout               time.Duration
	Logger                *logger.Logger
	SSHDialer             SSHDialer
	SSHPrivateKeyReader   func(path string) ([]byte, error)
	SSHPublicKeyReader    func(path string) ([]byte, error)
	ClientConfig          *ssh.ClientConfig
	InsecureIgnoreHostKey bool
}

type SSHConfiger interface {
	Connect() (SSHClienter, error)
	ExecuteCommand(command string) (string, error)
	PushFile(localPath, remotePath string) error
	InstallSystemdService(serviceName, serviceContent string) error
	StartService(serviceName string) error
	RestartService(serviceName string) error
}

func NewSSHConfig(
	host string,
	port int,
	user string,
	sshPrivateKeyPath string) (*SSHConfig, error) {
	// Confirm that the private key path exists
	if _, err := os.Stat(sshPrivateKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("private key path does not exist: %s", sshPrivateKeyPath)
	}

	// Open the private key file
	privateKeyFile, err := os.Open(sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open private key file: %w", err)
	}
	privateKeyBytes, err := io.ReadAll(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	defer privateKeyFile.Close()

	key, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	hostKeyCallback, err := GetHostKeyCallback(host)
	if err != nil {
		//nolint: gosec
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	sshClientConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         SSHTimeOut,
	}

	dialer := NewSSHDial(host, port, sshClientConfig)

	return &SSHConfig{
		Host:                  host,
		Port:                  port,
		User:                  user,
		PrivateKeyPath:        sshPrivateKeyPath,
		Timeout:               SSHTimeOut,
		Logger:                logger.Get(),
		ClientConfig:          sshClientConfig,
		SSHDialer:             dialer,
		InsecureIgnoreHostKey: false,
	}, nil
}

func (c *SSHConfig) Connect() (SSHClienter, error) {
	c.Logger.Infof("Connecting to SSH server: %s:%d", c.Host, c.Port)

	client, err := c.SSHDialer.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), c.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	return client, nil
}

func (c *SSHConfig) NewSession() (SSHSessioner, error) {
	client := NewSSHClientFunc(c.ClientConfig, c.SSHDialer)

	return client.NewSession()
}

func (c *SSHConfig) ExecuteCommand(command string) (string, error) {
	c.Logger.Infof("Executing command: %s", command)

	var output string
	err := retry(NumberOfSSHRetries, TimeInBetweenSSHRetries, func() error {
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

func (c *SSHConfig) PushFile(localPath, remotePath string) error {
	c.Logger.Infof("Pushing file: %s to %s", localPath, remotePath)

	session, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	remoteCmd := fmt.Sprintf("cat > %s", remotePath)
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

	// Open the local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Copy the local file contents to the remote command's stdin
	_, err = io.Copy(stdin, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
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
	serviceName, serviceContent string,
) error {
	c.Logger.Infof("Installing systemd service: %s", serviceName)
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

func (c *SSHConfig) StartService(serviceName string) error {
	return c.manageService(serviceName, "start")
}

func (c *SSHConfig) RestartService(serviceName string) error {
	return c.manageService(serviceName, "restart")
}

func (c *SSHConfig) manageService(serviceName, action string) error {
	c.Logger.Infof("Managing service: %s %s", serviceName, action)
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
