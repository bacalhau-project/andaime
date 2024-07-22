package sshutils

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

// SSHDial is a function type for creating SSH clients
type SSHClientCreator func(config *ssh.ClientConfig, dialer SSHDialer) SSHClienter

var NewSSHClientFunc SSHClientCreator = NewSSHClient

// SSHClienter interface defines the methods we need for SSH operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
}

type SSHClient struct {
	SSHClientConfig *ssh.ClientConfig
	Client          SSHClienter
	Dialer          SSHDialer
}

func NewSSHClient(sshClientConfig *ssh.ClientConfig, dialer SSHDialer) SSHClienter {
	return &SSHClient{SSHClientConfig: sshClientConfig, Dialer: dialer}
}

func (cl *SSHClient) Connect(network, addr string) error {
	client, err := cl.Dialer.Dial(network, addr, cl.SSHClientConfig)
	if err != nil {
		return err
	}
	cl.Client = client
	return nil
}

func (cl *SSHClient) NewSession() (SSHSessioner, error) {
	if cl.Client == nil {
		return nil, fmt.Errorf("SSH client not connected")
	}
	return cl.Client.NewSession()
}

func (cl *SSHClient) Close() error {
	if cl.Client == nil {
		return nil
	}
	return cl.Client.Close()
}
