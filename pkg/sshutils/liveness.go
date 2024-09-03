package sshutils

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	DefaultSSHPort    = 22
	DefaultRetryCount = 5
	DefaultRetryDelay = 10 * time.Second
)

type SSHLivenessChecker struct {
	RetryCount int
	RetryDelay time.Duration
}

func NewSSHLivenessChecker() *SSHLivenessChecker {
	return &SSHLivenessChecker{
		RetryCount: DefaultRetryCount,
		RetryDelay: DefaultRetryDelay,
	}
}

func (c *SSHLivenessChecker) CheckSSHLiveness(
	ctx context.Context,
	ip, user, privateKeyPath string,
	port int,
) error {
	sshConfig, err := getSSHConfig(user, privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to get SSH config: %v", err)
	}

	for i := 0; i < c.RetryCount; i++ {
		err = trySSHConnection(ctx, ip, port, sshConfig)
		if err == nil {
			return nil
		}
		time.Sleep(c.RetryDelay)
	}

	return fmt.Errorf("failed to establish SSH connection after %d attempts: %v", c.RetryCount, err)
}

func (c *SSHLivenessChecker) TestConnectivity(
	ip, user string,
	port int,
	privateKeyPath string,
) error {
	sshConfig, err := getSSHConfig(user, privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to get SSH config: %v", err)
	}

	for i := 0; i < c.RetryCount; i++ {
		err = testSSHConnection(ip, port, sshConfig)
		if err == nil {
			return nil
		}
		time.Sleep(c.RetryDelay)
	}

	return fmt.Errorf("failed to establish SSH connection after %d attempts: %v", c.RetryCount, err)
}

func getSSHConfig(user, privateKeyPath string) (*ssh.ClientConfig, error) {
	key, err := getPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %v", err)
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}

func getPrivateKey(privateKeyPath string) (ssh.Signer, error) {
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %v", err)
	}

	privateKey, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return privateKey, nil
}

func trySSHConnection(
	_ context.Context,
	ip string,
	port int,
	config *ssh.ClientConfig,
) error {
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), config)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return nil
}

func testSSHConnection(ip string, port int, config *ssh.ClientConfig) error {
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), config)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	output, err := session.CombinedOutput("echo 'SSH connection successful'")
	if err != nil {
		return fmt.Errorf("failed to execute command: %v", err)
	}

	if string(output) != "SSH connection successful\n" {
		return fmt.Errorf("unexpected output: %s", string(output))
	}

	return nil
}
