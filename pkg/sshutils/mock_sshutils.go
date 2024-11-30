package sshutils

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

type MockSSHClient struct {
	mock.Mock
}

func (m *MockSSHClient) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	args := m.Called(ctx, remotePath, content, executable)
	return args.Error(0)
}

func (m *MockSSHClient) NewSession() (SSHSessioner, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SSHSessioner), args.Error(1)
}

func (m *MockSSHClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockSSHClient) GetClient() *ssh.Client {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*ssh.Client)
}

func GetTypedMockClient(t *testing.T, log *logger.Logger) (*MockSSHClient, SSHConfiger) {
	mockSSHClient := &MockSSHClient{}
	sshConfig, err := NewSSHConfig("example.com", 22, "testuser", "/path/to/key")
	if err != nil {
		t.Fatalf("Failed to create mock SSH config: %v", err)
	}
	sshConfig.SetSSHClienter(mockSSHClient)
	return mockSSHClient, sshConfig
}

type MockWriteCloser struct {
	mock.Mock
}

func (m *MockWriteCloser) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockWriteCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockSessioner interface {
	Run(cmd string) error
	CombinedOutput(cmd string) ([]byte, error)
	StdinPipe() (io.WriteCloser, error)
	Start(cmd string) error
	Close() error
	Wait() error
}

type MockSSHSession struct {
	mock.Mock
}

func NewMockSSHSession() *MockSSHSession {
	return &MockSSHSession{}
}

func (m *MockSSHSession) Run(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSession) CombinedOutput(cmd string) ([]byte, error) {
	args := m.Called(cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSSHSession) StdinPipe() (io.WriteCloser, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (m *MockSSHSession) StdoutPipe() (io.Reader, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.Reader), args.Error(1)
}

func (m *MockSSHSession) StderrPipe() (io.Reader, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.Reader), args.Error(1)
}

func (m *MockSSHSession) Start(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSession) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHSession) Wait() error {
	args := m.Called()
	return args.Error(0)
}

var _ SSHSessioner = &MockSSHSession{}

type MockSSHConfig struct {
	mock.Mock
}

func (m *MockSSHConfig) Connect() (SSHClienter, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SSHClienter), args.Error(1)
}

func (m *MockSSHConfig) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	args := m.Called(ctx, retry, timeout)
	return args.Error(0)
}

func (m *MockSSHConfig) GetHost() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSSHConfig) GetPort() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockSSHConfig) GetUser() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSSHConfig) GetPrivateKeyMaterial() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *MockSSHConfig) GetSSHDial() SSHDialer {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(SSHDialer)
}

func (m *MockSSHConfig) SetSSHDial(dialer SSHDialer) {
	m.Called(dialer)
}

func (m *MockSSHConfig) GetSSHClienter() SSHClienter {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(SSHClienter)
}

func (m *MockSSHConfig) SetSSHClienter(client SSHClienter) {
	m.Called(client)
}

func (m *MockSSHConfig) GetSSHClient() *ssh.Client {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*ssh.Client)
}

func (m *MockSSHConfig) SetSSHClient(client *ssh.Client) {
	m.Called(client)
}

func (m *MockSSHConfig) SetValidateSSHConnection(fn func() error) {
	m.Called(fn)
}

func (m *MockSSHConfig) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHConfig) ExecuteCommand(ctx context.Context, cmd string) (string, error) {
	args := m.Called(ctx, cmd)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfig) ExecuteCommandWithCallback(
	ctx context.Context,
	cmd string,
	f func(string),
) (string, error) {
	args := m.Called(ctx, cmd, f)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfig) InstallSystemdService(
	ctx context.Context,
	servicePath string,
	serviceContent string,
) error {
	args := m.Called(ctx, servicePath, serviceContent)
	return args.Error(0)
}

func (m *MockSSHConfig) RemoveSystemdService(ctx context.Context, servicePath string) error {
	args := m.Called(ctx, servicePath)
	return args.Error(0)
}

func (m *MockSSHConfig) PushFile(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
) error {
	args := m.Called(ctx, remotePath, content, executable)
	return args.Error(0)
}

func (m *MockSSHConfig) PushFileWithCallback(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
	callback func(int64, int64),
) error {
	args := m.Called(ctx, remotePath, content, executable, callback)
	return args.Error(0)
}

func (m *MockSSHConfig) RestartService(
	ctx context.Context,
	servicePath string,
) error {
	args := m.Called(ctx, servicePath)
	return args.Error(0)
}

func (m *MockSSHConfig) StartService(
	ctx context.Context,
	servicePath string,
) error {
	args := m.Called(ctx, servicePath)
	return args.Error(0)
}

type MockSSHDialer struct {
	mock.Mock
}

func (m *MockSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
	args := m.Called(network, addr, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SSHClienter), args.Error(1)
}

func (m *MockSSHDialer) DialContext(
	ctx context.Context,
	network, addr string,
	config *ssh.ClientConfig,
) (SSHClienter, error) {
	args := m.Called(ctx, network, addr, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SSHClienter), args.Error(1)
}
