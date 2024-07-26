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
	PrivateKeyMaterial    string
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
	ExecuteCommand(client SSHClienter, command string) (string, error)
	PushFile(client SSHClienter, localPath, remotePath string) error
	InstallSystemdService(client SSHClienter, serviceName, serviceContent string) error
	StartService(client SSHClienter, serviceName string) error
	RestartService(client SSHClienter, serviceName string) error
}

func NewSSHConfig(host string, port int,
	user string,
	dialer SSHDialer,
	sshPrivateKeyPath string) (*SSHConfig, error) {
	// TODO: Implement GetSSHKeysFromPath function
	sshPrivateKey, err := os.ReadFile(sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH key: %w", err)
	}

	return &SSHConfig{
		Host:                  host,
		Port:                  port,
		User:                  user,
		PrivateKeyMaterial:    string(sshPrivateKey),
		Timeout:               SSHTimeOut,
		Logger:                logger.Get(),
		SSHDialer:             dialer,
		InsecureIgnoreHostKey: false,
	}, nil
}

func (c *SSHConfig) Connect() (SSHClienter, error) {
	c.Logger.Infof("Connecting to SSH server: %s:%d", c.Host, c.Port)

	key, err := ssh.ParsePrivateKey([]byte(c.PrivateKeyMaterial))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	var hostKeyCallback ssh.HostKeyCallback
	if c.InsecureIgnoreHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
	} else {
		hostKeyCallback, err = c.getHostKeyCallback()
		if err != nil {
			c.Logger.Warnf("Unable to get host key, falling back to insecure ignore: %v", err)
			//nolint: gosec
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	}
	sshConfig := &ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.Timeout,
	}

	client, err := c.SSHDialer.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	return client, nil
}

func (c *SSHConfig) ExecuteCommand(client SSHClienter, command string) (string, error) {
	c.Logger.Infof("Executing command: %s", command)

	var output string
	err := retry(NumberOfSSHRetries, TimeInBetweenSSHRetries, func() error {
		session, err := client.NewSession()
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

func (c *SSHConfig) PushFile(client SSHClienter, localPath, remotePath string) error {
	c.Logger.Infof("Pushing file: %s to %s", localPath, remotePath)

	session, err := client.NewSession()
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

func (c *SSHConfig) InstallSystemdService(client SSHClienter, serviceName, serviceContent string) error {
	c.Logger.Infof("Installing systemd service: %s", serviceName)
	remoteServicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	session, err := client.NewSession()
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

func (c *SSHConfig) StartService(client SSHClienter, serviceName string) error {
	return c.manageService(client, serviceName, "start")
}

func (c *SSHConfig) RestartService(client SSHClienter, serviceName string) error {
	return c.manageService(client, serviceName, "restart")
}

func (c *SSHConfig) manageService(client SSHClienter, serviceName, action string) error {
	c.Logger.Infof("Managing service: %s %s", serviceName, action)
	session, err := client.NewSession()
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
