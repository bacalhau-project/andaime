package sshutils

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"golang.org/x/crypto/ssh"
)

// SSHClienter interface defines the methods we need for SSH operations
type SSHClienter interface {
	NewSession() (SSHSessioner, error)
	IsConnected() bool
	Close() error
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

func (cl *SSHClient) IsConnected() bool {
	return cl.Client != nil && cl.Client.IsConnected()
}

type SSHClientWrapper struct {
	*ssh.Client
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

func (w *SSHClientWrapper) IsConnected() bool {
	l := logger.Get()
	if w.Client == nil {
		l.Debug("SSH client is nil")
		return false
	}

	// Check if the underlying network connection is still alive
	if _, err := w.Client.NewSession(); err != nil {
		l.Debugf("Failed to create new session, connection may be dead: %v", err)
		return false
	}

	// Try to create a new session
	session, err := w.Client.NewSession()
	if err != nil {
		l.Debugf("Failed to create SSH session: %v", err)
		return false
	}
	defer session.Close()

	// Run a simple command to check if the connection is alive
	l.Debug("Testing SSH connection with 'echo' command")
	err = session.Run("echo")
	if err != nil {
		l.Debugf("SSH connection test failed: %v", err)
		return false
	}

	l.Debug("SSH connection test successful")
	return true
}

type SSHSessionWrapper struct {
	Session *ssh.Session
}

// SSHError represents an SSH command execution error with output
type SSHError struct {
	Cmd    string
	Output string
	Err    error
}

func (e *SSHError) Error() string {
	return fmt.Sprintf("SSH command failed:\nCommand: %s\nOutput: %s\nError: %v",
		e.Cmd, e.Output, e.Err)
}

func (s *SSHSessionWrapper) Run(cmd string) error {
	l := logger.Get()
	if s.Session == nil {
		return fmt.Errorf("SSH session is nil")
	}

	l.Infof("Executing SSH command: %s", cmd)

	// Wrap the command in sudo bash -c to handle all parts in one go
	wrappedCmd := fmt.Sprintf("sudo bash -c '%s'", strings.Replace(cmd, "'", "'\"'\"'", -1))

	// For regular commands without stdin
	output, err := s.Session.CombinedOutput(cmd)
	if err != nil {
		l.Errorf("SSH command failed: %v", err)
		l.Errorf("Command output: %s", string(output))
		if len(output) == 0 {
			l.Debug("No command output received - this may indicate a connection failure")
			return &SSHError{
				Cmd: cmd,
				Err: fmt.Errorf(
					"SSH command failed with no output (possible connection failure): %w",
					err,
				),
			}
		}
		return &SSHError{
			Cmd:    cmd,
			Output: string(output),
			Err:    err,
		}
	}
	l.Infof("SSH command completed successfully")
	l.Debugf("Command output: %s", string(output))
	return nil
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
