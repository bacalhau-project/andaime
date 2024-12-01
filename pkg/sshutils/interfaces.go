package sshutils

import (
	"fmt"
	"io"

	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"golang.org/x/crypto/ssh"
)

// SSHClientWrapper wraps an ssh.Client
type SSHClientWrapper struct {
	Client *ssh.Client
}

func (w *SSHClientWrapper) Close() error {
	if w.Client != nil {
		return w.Client.Close()
	}
	return nil
}

func (w *SSHClientWrapper) NewSession() (sshutils_interfaces.SSHSessioner, error) {
	if w.Client == nil {
		return nil, fmt.Errorf("SSH client is nil")
	}
	session, err := w.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &SSHSessionWrapper{Session: session}, nil
}

func (w *SSHClientWrapper) GetClient() *ssh.Client {
	return w.Client
}

// SSHSessionWrapper wraps an ssh.Session
type SSHSessionWrapper struct {
	Session *ssh.Session
}

func (s *SSHSessionWrapper) StdinPipe() (io.WriteCloser, error) {
	return s.Session.StdinPipe()
}

func (s *SSHSessionWrapper) Start(cmd string) error {
	return s.Session.Start(cmd)
}

func (s *SSHSessionWrapper) Wait() error {
	return s.Session.Wait()
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
