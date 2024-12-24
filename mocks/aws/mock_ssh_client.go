package aws

import (
	"context"
	"golang.org/x/crypto/ssh"
)

// MockSSHClient implements SSHClienter interface for testing
type MockSSHClient struct {
	ConnectFunc           func() error
	CloseFunc            func() error
	ExecuteCommandFunc    func(ctx context.Context, command string) (string, error)
	GetHostFunc          func() string
	GetPortFunc          func() int
	GetUserFunc          func() string
	GetPrivateKeyMaterialFunc func() []byte
	GetSSHClientFunc     func() *ssh.Client
	SetSSHClientFunc     func(client *ssh.Client)
	IsConnectedFunc      func() bool
}

func (m *MockSSHClient) Connect() error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc()
	}
	return nil
}

func (m *MockSSHClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockSSHClient) ExecuteCommand(ctx context.Context, command string) (string, error) {
	if m.ExecuteCommandFunc != nil {
		return m.ExecuteCommandFunc(ctx, command)
	}
	return "", nil
}

func (m *MockSSHClient) GetHost() string {
	if m.GetHostFunc != nil {
		return m.GetHostFunc()
	}
	return "localhost"
}

func (m *MockSSHClient) GetPort() int {
	if m.GetPortFunc != nil {
		return m.GetPortFunc()
	}
	return 22
}

func (m *MockSSHClient) GetUser() string {
	if m.GetUserFunc != nil {
		return m.GetUserFunc()
	}
	return "test"
}

func (m *MockSSHClient) GetPrivateKeyMaterial() []byte {
	if m.GetPrivateKeyMaterialFunc != nil {
		return m.GetPrivateKeyMaterialFunc()
	}
	return []byte("test-key")
}

func (m *MockSSHClient) GetSSHClient() *ssh.Client {
	if m.GetSSHClientFunc != nil {
		return m.GetSSHClientFunc()
	}
	return nil
}

func (m *MockSSHClient) SetSSHClient(client *ssh.Client) {
	if m.SetSSHClientFunc != nil {
		m.SetSSHClientFunc(client)
	}
}

func (m *MockSSHClient) IsConnected() bool {
	if m.IsConnectedFunc != nil {
		return m.IsConnectedFunc()
	}
	return true
}
