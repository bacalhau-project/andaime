package sshutils

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type SSHWaiter interface {
	WaitForSSH(publicIP, username string, privateKey []byte) error
}

var SSHWaiterFunc = WaitForSSHToBeLive

type DefaultSSHWaiter struct {
	Client SSHClienter
}

func NewSSHWaiter(client SSHClienter) *DefaultSSHWaiter {
	return &DefaultSSHWaiter{Client: client}
}

func (w *DefaultSSHWaiter) WaitForSSH(config *SSHConfig) error {
	return WaitForSSHToBeLive(config, SSHRetryAttempts, SSHRetryDelay)
}

// WaitForSSHToBeLive attempts to establish an SSH connection to the VM
func WaitForSSHToBeLive(config *SSHConfig, retries int, delay time.Duration) error {
	l := logger.Get()
	if config == nil {
		err := fmt.Errorf("SSH config is nil")
		l.Error(err.Error())
		return err
	}
	l.Debugf("Starting SSH connection check to %s:%d", config.Host, config.Port)
	l.Debug("Entering waitForSSH")
	l.Debugf(
		"publicIP: %s, username: %s, privateKeyPath: %s\n",
		config.Host,
		config.User,
		config.PrivateKeyPath,
	)

	// Open the private key file
	privateKey, err := os.Open(config.PrivateKeyPath)
	if err != nil {
		return err
	}
	defer privateKey.Close()

	// Get the private key
	privateKeyBytes, err := io.ReadAll(privateKey)
	if err != nil {
		return err
	}

	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		err = fmt.Errorf("failed to parse private key in waitForSSH: %v", err)
		l.Error(err.Error())
		return err
	}
	l.Debug("Private key parsed successfully")

	sshClientConfig := &ssh.ClientConfig{
		User: config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         SSHTimeOut,
	}
	dialer := NewSSHDial(config.Host, config.Port, sshClientConfig)
	andaimeSSHClientConfig, err := NewSSHConfig(
		config.Host,
		config.Port,
		config.User,
		dialer,
		config.PrivateKeyPath,
	)
	if err != nil {
		err = fmt.Errorf("failed to create SSH client config: %v", err)
		l.Error(err.Error())
		return err
	}
	l.Debug("SSH client config created")

	for i := 0; i < SSHRetryAttempts; i++ {
		l.Debugf("Attempt %d to connect via SSH\n", i+1)
		client, err := andaimeSSHClientConfig.Connect()
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

	err = fmt.Errorf("failed to establish SSH connection after multiple attempts")
	l.Error(err.Error())
	return err
}
