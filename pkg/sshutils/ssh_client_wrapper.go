package sshutils

import "golang.org/x/crypto/ssh"

type SSHClientWrapper struct {
	Client *ssh.Client
}

func (c *SSHClientWrapper) NewSession() (SSHSessioner, error) {
	session, err := c.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &SSHSessionWrapper{Session: session}, nil
}

func (c *SSHClientWrapper) Close() error {
	return c.Client.Close()
}
