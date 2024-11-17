package sshutils

import (
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

	// Keep a reference to the session for cleanup
	session := s.Session
	defer func() {
		if session != nil {
			session.Close()
		}
	}()

	// For file transfer commands that contain a pipe, we need to handle stdin
	if strings.Contains(wrappedCmd, "cat >") {
		stdin, err := session.StdinPipe()
		if err != nil {
			return &SSHError{
				Cmd: cmd,
				Err: fmt.Errorf("failed to get stdin pipe: %w", err),
			}
		}

		// Set up pipes for stderr only since we're using stdin for content
		var stderrBuf strings.Builder
		stderr, err := session.StderrPipe()
		if err != nil {
			return &SSHError{
				Cmd: cmd,
				Err: fmt.Errorf("failed to get stderr pipe: %w", err),
			}
		}

		// Copy stderr in background
		go func() {
			_, err := io.Copy(&stderrBuf, stderr)
			if err != nil {
				l.Errorf("failed to copy stderr: %v", err)
			}
		}()

		l.Debugf("Starting SSH command with wrapped command: %s", wrappedCmd)

		// Start the command before writing to stdin
		if err := session.Start(wrappedCmd); err != nil {
			l.Errorf("Failed to start SSH command: %v", err)
			session.Close()
			return &SSHError{
				Cmd: cmd,
				Err: fmt.Errorf("failed to start command: %w", err),
			}
		}

		// Extract content from the command (assuming it's between the last '>' and any optional chmod)
		parts := strings.Split(cmd, ">")
		if len(parts) < 2 {
			return fmt.Errorf("invalid file transfer command format")
		}
		content := strings.TrimSpace(parts[1])
		if idx := strings.Index(content, "&&"); idx != -1 {
			content = strings.TrimSpace(content[:idx])
		}

		// Write the content to stdin and close it
		_, err = io.WriteString(stdin, content)
		if err != nil {
			l.Errorf("Failed to write to stdin: %v", err)
			return &SSHError{
				Cmd: cmd,
				Err: fmt.Errorf("failed to write content: %w", err),
			}
		}
		stdin.Close()

		l.Debug("Waiting for SSH command completion...")
		err = session.Wait()
		if err != nil {
			l.Errorf("SSH command failed: %v", err)
			return &SSHError{
				Cmd:    cmd,
				Output: stderrBuf.String(),
				Err:    fmt.Errorf("command failed: %w", err),
			}
		}
		return nil
	} else {
		// For non-file transfer commands, use combined output with timeout handling
		var output []byte
		var err error

		// Try command multiple times with increasing timeouts
		timeouts := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second}
		for i, timeout := range timeouts {
			done := make(chan struct{})
			go func() {
				output, err = session.CombinedOutput(wrappedCmd)
				close(done)
			}()

			select {
			case <-done:
				if err == nil {
					l.Debugf("Command output: %s", string(output))
					return nil
				}
				if i == len(timeouts)-1 {
					l.Errorf("SSH command failed after all retries: %v", err)
					l.Errorf("Command output: %s", string(output))
					return &SSHError{
						Cmd:    cmd,
						Output: string(output),
						Err:    fmt.Errorf("command failed after %d retries: %w", len(timeouts), err),
					}
				}
				l.Warnf("Command failed with timeout %v, retrying with longer timeout", timeout)
			case <-time.After(timeout):
				if i == len(timeouts)-1 {
					l.Errorf("Command timed out after %v on final attempt", timeout)
					return &SSHError{
						Cmd: cmd,
						Err: fmt.Errorf("command timed out after %v on final attempt", timeout),
					}
				}
				l.Warnf("Command timed out after %v, retrying with longer timeout", timeout)
				session.Close()
				session, err = cl.NewSession()
				if err != nil {
					return &SSHError{
						Cmd: cmd,
						Err: fmt.Errorf("failed to create new session after timeout: %w", err),
					}
				}
			}
		}
		return fmt.Errorf("unexpected exit from retry loop")
	}
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
