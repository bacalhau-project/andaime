package sshutils

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

// SSHClienter interface defines the methods we need for SSH operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
	TestConnectivity(ip, user string, port int, privateKeyPath string) error
}

type SSHClient struct {
	SSHClientConfig *ssh.ClientConfig
	Client          SSHClienter
	Dialer          SSHDialer
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

func (cl *SSHClient) TestConnectivity(ip, user string, port int, privateKeyPath string) error {
	checker := NewSSHLivenessChecker()
	return checker.TestConnectivity(ip, user, port, privateKeyPath)
}

type SSHClientWrapper struct {
	*ssh.Client
}

// TestConnectivity implements SSHClienter.
func (w *SSHClientWrapper) TestConnectivity(
	ip string,
	user string,
	port int,
	privateKeyPath string,
) error {
	panic("unimplemented")
}

func (w *SSHClientWrapper) NewSession() (SSHSessioner, error) {
	session, err := w.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &SSHSessionWrapper{session}, nil
}

func (w *SSHClientWrapper) Close() error {
	return w.Client.Close()
}
