package sshutils

import (
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
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

	andaimeSSHClientConfig, err := NewSSHConfig(
		config.Host,
		config.Port,
		config.User,
		config.PrivateKeyPath,
	)
	if err != nil {
		l.Errorf("error creating client: %v", err)
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
