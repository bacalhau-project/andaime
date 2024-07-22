package sshutils

import "io"

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
