package sshutils

import (
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHSessionWrapper implements SSHSessioner interface
type SSHSessionWrapper struct {
	Session *ssh.Session
}

func (s *SSHSessionWrapper) Run(cmd string) error {
	return s.Session.Run(cmd)
}

func (s *SSHSessionWrapper) CombinedOutput(cmd string) ([]byte, error) {
	return s.Session.CombinedOutput(cmd)
}

func (s *SSHSessionWrapper) Close() error {
	return s.Session.Close()
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

func (s *SSHSessionWrapper) Signal(sig ssh.Signal) error {
	return s.Session.Signal(sig)
}
