package sshutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/logger"
	"github.com/bacalhau-project/andaime/utils"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	Host             string
	Port             int
	User             string
	PrivateKey       string
	Timeout          time.Duration
	Logger           *logger.Logger
	SSHClientFactory SSHClientFactory
}

func NewSSHConfig(host string, port int, user string) (*SSHConfig, error) {
	privateKey, err := getSSHKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH key: %w", err)
	}

	return &SSHConfig{
		Host:             host,
		Port:             port,
		User:             user,
		PrivateKey:       privateKey,
		Timeout:          utils.SSHTimeOut,
		Logger:           logger.Get(),
		SSHClientFactory: &DefaultSSHClientFactory{},
	}, nil
}

type SSHClientFactory interface {
	NewSSHClient(network, addr string, config *ssh.ClientConfig) (SSHClient, error)
}

type sshClientWrapper struct {
	*ssh.Client
}

func (w *sshClientWrapper) NewSession() (SSHSession, error) {
	session, err := w.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &sshSessionWrapper{Session: session}, nil
}

type sshSessionWrapper struct {
	*ssh.Session
}

type DefaultSSHClientFactory struct{}

func (f *DefaultSSHClientFactory) NewSSHClient(network, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	client, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, err
	}
	return &sshClientWrapper{Client: client}, nil
}

func (c *SSHConfig) Connect() (SSHClient, error) {
	c.Logger.Info("Connecting to SSH server",
		logger.String("host", c.Host),
		logger.Int("port", c.Port),
		logger.String("user", c.User),
	)

	key, err := ssh.ParsePrivateKey([]byte(c.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	hostKeyCallback, err := c.getHostKeyCallback()
	if err != nil {
		c.Logger.Warn("Unable to get host key, falling back to insecure ignore",
			logger.Error(err),
		)
		//nolint: gosec
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
	config := &ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.Timeout,
	}

	return c.SSHClientFactory.NewSSHClient("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), config)
}

func (c *SSHConfig) ExecuteCommand(client SSHClient, command string) (string, error) {
	c.Logger.Info("Executing command:",
		logger.String("command", command),
	)

	var output string
	err := retry(utils.NumberOfSSHRetries, utils.TimeInBetweenSSHRetries, func() error {
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

func (c *SSHConfig) PushFile(client SSHClient, localPath, remotePath string) error {
	c.Logger.Info("Pushing file",
		logger.String("localPath", localPath),
		logger.String("remotePath", remotePath),
	)
	sshClient, ok := client.(*sshClientWrapper)
	if !ok {
		return fmt.Errorf("invalid client type")
	}

	sftpClient, err := sftp.NewClient(sshClient.Client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

func (c *SSHConfig) InstallSystemdService(client SSHClient, serviceName, serviceContent string) error {
	c.Logger.Info("Installing systemd service",
		logger.String("serviceName", serviceName),
	)
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

func (c *SSHConfig) StartService(client SSHClient, serviceName string) error {
	return c.manageService(client, serviceName, "start")
}

func (c *SSHConfig) RestartService(client SSHClient, serviceName string) error {
	return c.manageService(client, serviceName, "restart")
}

func (c *SSHConfig) manageService(client SSHClient, serviceName, action string) error {
	c.Logger.Info("Managing service",
		logger.String("serviceName", serviceName),
		logger.String("action", action),
	)
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

func retry(attempts int, sleep time.Duration, f func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %w", attempts, err)
}

func getSSHKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	keyPath := filepath.Join(home, ".ssh", "id_rsa")
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH key file: %w", err)
	}

	return string(keyBytes), nil
}

func getHostKey(host string) (ssh.PublicKey, error) {
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("failed to open known_hosts file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		numberOfPublicKeyFields := 3
		if len(fields) != numberOfPublicKeyFields {
			continue
		}
		if strings.Contains(fields[0], host) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				return nil, fmt.Errorf("failed to parse host key: %w", err)
			}
			break
		}
	}

	if hostKey == nil {
		return nil, fmt.Errorf("no hostkey found for %s", host)
	}

	return hostKey, nil
}

func (c *SSHConfig) getHostKeyCallback() (ssh.HostKeyCallback, error) {
	hostKey, err := getHostKey(c.Host)
	if err != nil {
		return nil, err
	}
	return ssh.FixedHostKey(hostKey), nil
}
