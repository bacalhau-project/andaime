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

func (s *SSHSessionWrapper) Run(cmd string) error {
	l := logger.Get()
	if s.Session == nil {
		l.Error("SSH session is nil")
		return fmt.Errorf("SSH session is nil")
	}

	l.Infof("Executing SSH command: %s", cmd)
	defer l.Sync() // Ensure logs are flushed

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
		l.Sync() // Ensure error logs are flushed
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

	l.Debug("Starting command execution monitoring")
	err = l.Sync()
	if err != nil {
		l.Errorf("Failed to sync logger: %v", err)
	}

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
		l.Error("Invalid file transfer command format")
		return fmt.Errorf("invalid file transfer command format")
	}

	content := strings.TrimSpace(parts[1])
	if idx := strings.Index(content, "&&"); idx != -1 {
		content = strings.TrimSpace(content[:idx])
	}

	l.Debugf("Attempting to write content (length: %d) to remote file", len(content))

	written, err := io.WriteString(stdin, content)
	if err != nil {
		l.Errorf("Failed to write content: %v", err)
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to write content: %w", err),
		}
	}
	l.Debugf("Successfully wrote %d bytes", written)

	if err := stdin.Close(); err != nil {
		l.Errorf("Error closing stdin: %v", err)
		return &SSHError{
			Cmd: cmd,
			Err: fmt.Errorf("failed to close stdin: %w", err),
		}
	}
	l.Debug("Successfully closed stdin")

	return s.Session.Wait()
}

func (s *SSHSessionWrapper) CombinedOutput(cmd string) ([]byte, error) {
	return s.Session.CombinedOutput(cmd)
}

func (s *SSHSessionWrapper) Close() error {
	if s.Session != nil {
		return s.Session.Close()
	}
	return nil
}

// Removed type declarations
