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
	"golang.org/x/crypto/ssh"
)

type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

type SSHConfig struct {
	Host             string
	Port             int
	User             string
	PrivateKey       string
	Timeout          time.Duration
	Logger           *logger.Logger
	SSHClientFactory SSHClientFactory
	SSHDialer        SSHDialer
}

func NewSSHConfig(host string, port int, user string, dialer SSHDialer, sshKeyPath string) (*SSHConfig, error) {
	_, privateKey, err := utils.GetSSHKeysFromPath(sshKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH key: %w", err)
	}

	return &SSHConfig{
		Host:             host,
		Port:             port,
		User:             user,
		PrivateKey:       string(privateKey),
		Timeout:          utils.SSHTimeOut,
		Logger:           logger.Get(),
		SSHDialer:        dialer,
		SSHClientFactory: NewDefaultSSHClientFactory(dialer),
	}, nil
}

type SSHClientFactory interface {
	NewSSHClient(network, addr string, config *ssh.ClientConfig, dialer SSHDialer) (SSHClient, error)
}

type SSHClientWrapper struct {
	Client *ssh.Client
}

func (w *SSHClientWrapper) NewSession() (SSHSession, error) {
	session, err := w.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &SSHSessionWrapper{Session: session}, nil
}

func (w *SSHClientWrapper) Close() error {
	return w.Client.Close()
}

type SSHSessionWrapper struct {
	*ssh.Session
}

type DefaultSSHClientFactory struct {
	dialer SSHDialer
}

func NewDefaultSSHClientFactory(dialer SSHDialer) *DefaultSSHClientFactory {
	return &DefaultSSHClientFactory{dialer: dialer}
}

func (f *DefaultSSHClientFactory) NewSSHClient(network, addr string, config *ssh.ClientConfig, dialer SSHDialer) (SSHClient, error) {
	client, err := dialer.Dial(network, addr, config)
	if err != nil {
		return nil, err
	}
	return &SSHClientWrapper{Client: client}, nil
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

	return c.SSHClientFactory.NewSSHClient("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), config, c.SSHDialer)
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

type DefaultSSHDialer struct{}

func (d *DefaultSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return ssh.Dial(network, addr, config)
}
