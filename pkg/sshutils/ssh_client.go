package sshutils

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

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
	session, err := w.Client.NewSession()
	if err != nil {
		l.Debugf("Failed to create new session, connection may be dead: %v", err)
		if strings.Contains(err.Error(), "use of closed network connection") {
			l.Error("SSH connection has been closed")
		} else if strings.Contains(err.Error(), "i/o timeout") {
			l.Error("SSH connection timed out")
		} else {
			l.Errorf("Unexpected SSH error: %v", err)
		}
		return false
	}
	defer session.Close()

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

	// Keep a reference to the session for cleanup
	session := s.Session
	defer func() {
		if session != nil {
			session.Close()
		}
	}()

	// For file transfer commands that contain a pipe, handle them separately
	if strings.Contains(wrappedCmd, "cat >") {
		return s.handleFileTransfer(cmd, wrappedCmd)
	}

	// Set up pipes for stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := session.Start(wrappedCmd); err != nil {
		l.Errorf("Failed to start SSH command: %s", wrappedCmd)
		l.Debugf("SSH command start error details: %v", err)
		return &SSHError{
			Cmd:    cmd,
			Output: "Command failed to start",
			Err:    fmt.Errorf("failed to start command: %w", err),
		}
	}
	l.Debugf("Successfully started SSH command: %s", wrappedCmd)

	// Channel to track the last activity time
	lastActivity := make(chan time.Time, 1)
	done := make(chan error, 1)

	// Start monitoring stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			l.Infof("stdout: %s", line)
			lastActivity <- time.Now()
		}
		if err := scanner.Err(); err != nil {
			l.Errorf("Error reading stdout: %v", err)
		}
		l.Debug("Stdout monitoring completed")
	}()

	// Start monitoring stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			l.Infof("stderr: %s", line)
			lastActivity <- time.Now()
		}
	}()

	// Start a goroutine to wait for the command completion
	go func() {
		done <- session.Wait()
	}()

	// Initialize the last activity time
	lastActivityTime := time.Now()
	const inactivityTimeout = 30 * time.Second

	// Monitor for completion or timeout
	for {
		select {
		case err := <-done:
			return err
		case t := <-lastActivity:
			lastActivityTime = t
		case <-time.After(1 * time.Second): // Check activity every second
			if time.Since(lastActivityTime) > inactivityTimeout {
				l.Warnf("Command inactive for %v, initiating termination", inactivityTimeout)
				
				// Try graceful termination first
				l.Debug("Sending SIGTERM signal")
				if err := session.Signal(ssh.SIGTERM); err != nil {
					l.Errorf("Error sending SIGTERM: %v", err)
				}
				
				time.Sleep(5 * time.Second) // Give it 5 seconds to cleanup
				
				// Force kill if still running
				l.Debug("Sending SIGKILL signal")
				if err := session.Signal(ssh.SIGKILL); err != nil {
					l.Errorf("Error sending SIGKILL: %v", err)
				}
				
				return &SSHError{
					Cmd:    cmd,
					Output: "Command timed out due to inactivity",
					Err: fmt.Errorf(
						"command terminated due to %v of inactivity - last activity at %v",
						inactivityTimeout,
						lastActivityTime.Format(time.RFC3339),
					),
				}
			}
		}
	}
}

// handleFileTransfer handles the special case of file transfer commands
func (s *SSHSessionWrapper) handleFileTransfer(cmd, wrappedCmd string) error {
	l := logger.Get()
	stdin, err := s.Session.StdinPipe()
	if err != nil {
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to get stdin pipe: %w", err),
		}
	}

	var stderrBuf strings.Builder
	stderr, err := s.Session.StderrPipe()
	if err != nil {
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to get stderr pipe: %w", err),
		}
	}

	// Copy stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			l.Infof("stderr: %s", line)
			stderrBuf.WriteString(line + "\n")
		}
	}()

	if err := s.Session.Start(wrappedCmd); err != nil {
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to start command: %w", err),
		}
	}

	// Extract and write content
	parts := strings.Split(cmd, ">")
	if len(parts) < 2 {
		return fmt.Errorf("invalid file transfer command format")
	}
	content := strings.TrimSpace(parts[1])
	if idx := strings.Index(content, "&&"); idx != -1 {
		content = strings.TrimSpace(content[:idx])
	}

	if _, err := io.WriteString(stdin, content); err != nil {
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to write content: %w", err),
		}
	}
	stdin.Close()

	return s.Session.Wait()
}

func (s *SSHSessionWrapper) Start(cmd string) error {
	return s.Session.Start(cmd)
}

func (s *SSHSessionWrapper) Wait() error {
	return s.Session.Wait()
}

func (s *SSHSessionWrapper) Close() error {
	if s.Session != nil {
		return s.Session.Close()
	}
	return nil
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
	if s.Session == nil {
		return nil, fmt.Errorf("ssh session is nil")
	}
	output, err := s.Session.CombinedOutput(cmd)
	if err != nil {
		return output, err
	}
	return output, nil
}
