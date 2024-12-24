package aws

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"golang.org/x/crypto/ssh"
)

// MockSSHClient is a custom mock implementation of SSHClienter with function fields
type MockSSHClient struct {
	ConnectFunc        func() (sshutils.SSHClienter, error)
	ExecuteCommandFunc func(ctx context.Context, command string) (string, error)
	IsConnectedFunc    func() bool
	CloseFunc         func() error
	GetClientFunc     func() *ssh.Client
	NewSessionFunc    func() (sshutils.SSHSessioner, error)
}

func (m *MockSSHClient) Connect() (sshutils.SSHClienter, error) {
	if m.ConnectFunc != nil {
		return m.ConnectFunc()
	}
	return nil, nil
}

func (m *MockSSHClient) ExecuteCommand(ctx context.Context, command string) (string, error) {
	if m.ExecuteCommandFunc != nil {
		return m.ExecuteCommandFunc(ctx, command)
	}
	return "", nil
}

func (m *MockSSHClient) IsConnected() bool {
	if m.IsConnectedFunc != nil {
		return m.IsConnectedFunc()
	}
	return false
}

func (m *MockSSHClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockSSHClient) GetClient() *ssh.Client {
	if m.GetClientFunc != nil {
		return m.GetClientFunc()
	}
	return nil
}

func (m *MockSSHClient) NewSession() (sshutils.SSHSessioner, error) {
	if m.NewSessionFunc != nil {
		return m.NewSessionFunc()
	}
	return nil, nil
}
