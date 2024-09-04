package sshutils

import (
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHSessioner interface defines the methods we need for SSH sessions
type SSHSessioner interface {
	Run(cmd string) error
	Start(cmd string) error
	Wait() error
	Close() error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
	CombinedOutput(cmd string) ([]byte, error)
}

// SSHClienter interface defines the methods we need for SSH operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	Close() error
	TestConnectivity(ip, user string, port int, privateKeyPath string) error
}

// SSHDialer interface defines the method for dialing SSH connections
type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error)
}

// SSHClient struct definition
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
	if checker == nil {
		return fmt.Errorf("failed to create SSH liveness checker")
	}
	return checker.TestConnectivity(ip, user, port, privateKeyPath)
}

// SSHLivenessChecker structure for checking SSH liveness
type SSHLivenessChecker struct {
	// Add necessary fields
}

// NewSSHLivenessChecker creates a new SSHLivenessChecker
func NewSSHLivenessChecker() *SSHLivenessChecker {
	return &SSHLivenessChecker{}
}

// TestConnectivity tests the SSH connectivity
func (c *SSHLivenessChecker) TestConnectivity(ip, user string, port int, privateKeyPath string) error {
	// Implement the connectivity test
	return nil
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
	return &SSHSessionWrapper{Session: session}, nil
}

func (w *SSHClientWrapper) Close() error {
	return w.Client.Close()
}

type SSHSessionWrapper struct {
	Session *ssh.Session
}

func (s *SSHSessionWrapper) Run(cmd string) error {
	return s.Session.Run(cmd)
}

func (s *SSHSessionWrapper) Start(cmd string) error {
	return s.Session.Start(cmd)
}

func (s *SSHSessionWrapper) Wait() error {
	return s.Session.Wait()
}

func (s *SSHSessionWrapper) Close() error {
	return s.Session.Close()
}

func (s *SSHSessionWrapper) StdinPipe() (io.WriteCloser, error) {
	return s.Session.StdinPipe()
}

func (s *SSHSessionWrapper) StdoutPipe() (io.Reader, error) {
	return s.Session.StdoutPipe()
}

func (s *SSHSessionWrapper) StderrPipe() (io.Reader, error) {
	return s.Session.StderrPipe()
}

func (s *SSHSessionWrapper) CombinedOutput(cmd string) ([]byte, error) {
	return s.Session.CombinedOutput(cmd)
}
