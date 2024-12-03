package sshutils

import (
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHSessionWrapper implements SSHSessioner interface
type SSHSessionWrapper struct {
	Session *ssh.Session
	Client  *ssh.Client
}

func (s *SSHSessionWrapper) Run(cmd string) error {
	if s.Session != nil {
		s.Session.Close()
		s.Session = nil
	}

	var err error
	s.Session, err = s.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create new session: %w", err)
	}
	defer func() {
		s.Session.Close()
		s.Session = nil
	}()
	return s.Session.Run(cmd)
}

func (s *SSHSessionWrapper) CombinedOutput(cmd string) ([]byte, error) {
	if s.Session != nil {
		s.Session.Close()
		s.Session = nil
	}

	var err error
	s.Session, err = s.Client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}
	defer func() {
		s.Session.Close()
		s.Session = nil
	}()
	return s.Session.CombinedOutput(cmd)
}

func (s *SSHSessionWrapper) Close() error {
	if s.Session != nil {
		err := s.Session.Close()
		s.Session = nil
		if err != nil {
			return fmt.Errorf("failed to close session: %w", err)
		}
	}
	if s.Client != nil {
		err := s.Client.Close()
		s.Client = nil
		if err != nil {
			return fmt.Errorf("failed to close client: %w", err)
		}
	}
	return nil
}

func (s *SSHSessionWrapper) StdinPipe() (io.WriteCloser, error) {
	if s.Session != nil {
		s.Session.Close()
		s.Session = nil
	}

	var err error
	s.Session, err = s.Client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}
	return s.Session.StdinPipe()
}

func (s *SSHSessionWrapper) Start(cmd string) error {
	if s.Session != nil {
		s.Session.Close()
		s.Session = nil
	}

	var err error
	s.Session, err = s.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create new session: %w", err)
	}
	return s.Session.Start(cmd)
}

func (s *SSHSessionWrapper) Wait() error {
	if s.Session == nil {
		return fmt.Errorf("no active session to wait on")
	}
	err := s.Session.Wait()
	if err != nil {
		return fmt.Errorf("session wait failed: %w", err)
	}
	return nil
}

func (s *SSHSessionWrapper) Signal(sig ssh.Signal) error {
	if s.Session == nil {
		return fmt.Errorf("no active session to signal")
	}
	err := s.Session.Signal(sig)
	if err != nil {
		return fmt.Errorf("session signal failed: %w", err)
	}
	return nil
}
