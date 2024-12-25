package types

import (
"golang.org/x/crypto/ssh"
)

// SSHSessioner defines the interface for SSH sessions
type SSHSessioner interface {
Close() error
RequestPty(term string, height, width int, termModes ssh.TerminalModes) error
Run(command string) error
Start(command string) error
Wait() error
StdinPipe() (ssh.Channel, error)
StdoutPipe() (ssh.Channel, error)
StderrPipe() (ssh.Channel, error)
}

// SSHClienter defines the interface for SSH clients
type SSHClienter interface {
Close() error
NewSession() (SSHSessioner, error)
}

// SSHClientCreator defines the interface for creating SSH clients
type SSHClientCreator interface {
NewClient(config *ssh.ClientConfig, addr string) (SSHClienter, error)
}
