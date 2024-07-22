package sshutils

import (
	"io"

	"golang.org/x/crypto/ssh"
)

type SSHSessionWrapper struct {
	Session *ssh.Session
}

func (s *SSHSessionWrapper) Start(cmd string) error {
	return s.Session.Start(cmd)
}

// Implement SSHSessioner methods for SSHSessionWrapper
func (s *SSHSessionWrapper) Run(cmd string) error {
	return s.Session.Run(cmd)
}

func (s *SSHSessionWrapper) CombinedOutput(cmd string) ([]byte, error) {
	return s.Session.CombinedOutput(cmd)
}

func (s *SSHSessionWrapper) Close() error {
	return s.Session.Close()
}

func (s *SSHSessionWrapper) Wait() error {
	return s.Session.Wait()
}

func (s *SSHSessionWrapper) Signal(sig ssh.Signal) error {
	return s.Session.Signal(sig)
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
