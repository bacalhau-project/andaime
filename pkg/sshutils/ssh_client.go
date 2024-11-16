package sshutils

import (
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

	// Wrap the command in sudo bash -c to handle all parts in one go, with proper escaping
	escapedCmd := strings.Replace(cmd, "'", "'\"'\"'", -1)
	escapedCmd = strings.Replace(escapedCmd, "\\", "\\\\", -1)
	wrappedCmd := fmt.Sprintf("sudo bash -c '%s'", escapedCmd)

	// Set up pipes for capturing output
	var stdoutBuf, stderrBuf strings.Builder
	stdout, err := s.Session.StdoutPipe()
	if err != nil {
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to get stdout pipe: %w", err),
		}
	}
	stderr, err := s.Session.StderrPipe()
	if err != nil {
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to get stderr pipe: %w", err),
		}
	}

	l.Debugf("Current file permissions at destination: %s", cmd)
	permCmd := fmt.Sprintf("ls -l %s 2>/dev/null || echo 'File does not exist'", strings.Split(cmd, " ")[3])
	perms, _ := s.Session.CombinedOutput(permCmd)
	l.Debugf("File permissions check output: %s", string(perms))

	// Start copying output in background
	go func() {
		_, err := io.Copy(&stdoutBuf, stdout)
		if err != nil {
			l.Errorf("failed to copy stdout: %v", err)
		}
	}()
	go func() {
		_, err := io.Copy(&stderrBuf, stderr)
		if err != nil {
			l.Errorf("failed to copy stderr: %v", err)
		}
	}()

	l.Debugf("Starting SSH command with wrapped command: %s", wrappedCmd)
	
	// Check if session is valid
	if s.Session == nil {
		l.Error("SSH session is nil before command execution")
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("ssh session is nil"),
		}
	}

	// Start the command
	if err := s.Session.Start(wrappedCmd); err != nil {
		l.Errorf("Failed to start SSH command: %v", err)
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to start command: %w", err),
		}
	}

	l.Debug("Waiting for SSH command completion...")
	// Wait for command completion
	err = s.Session.Wait()
	l.Debugf("SSH command wait completed with error: %v", err)
	if err != nil {
		l.Errorf("SSH command failed: %v", err)
		l.Errorf("STDOUT: %s", stdoutBuf.String())
		l.Errorf("STDERR: %s", stderrBuf.String())

		return &SSHError{
			Cmd:    cmd,
			Output: fmt.Sprintf("STDOUT:\n%s\nSTDERR:\n%s", stdoutBuf.String(), stderrBuf.String()),
			Err:    fmt.Errorf("command failed with exit code %v: %w", err, err),
		}
	}

	output := stdoutBuf.String()
	l.Infof("SSH command completed successfully")
	l.Debugf("Command output:\nSTDOUT: %s\nSTDERR: %s", output, stderrBuf.String())
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
