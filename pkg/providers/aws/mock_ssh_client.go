package aws

import (
	"context"
	"io"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

// MockSSHSession implements the SSHSessioner interface for testing
type MockSSHSession struct {
	mock.Mock
	stderr io.Writer
	stdout io.Writer
}

func (m *MockSSHSession) Run(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSession) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHSession) CombinedOutput(cmd string) ([]byte, error) {
	args := m.Called(cmd)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSSHSession) SetStderr(w io.Writer) {
	m.stderr = w
	m.Called(w)
}

func (m *MockSSHSession) SetStdout(w io.Writer) {
	m.stdout = w
	m.Called(w)
}

func (m *MockSSHSession) StdinPipe() (io.WriteCloser, error) {
	args := m.Called()
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (m *MockSSHSession) Start(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSession) Wait() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHSession) Signal(sig ssh.Signal) error {
	args := m.Called(sig)
	return args.Error(0)
}

// MockSSHClient implements the SSHConfiger interface for testing
type MockSSHClient struct {
	mock.Mock
	connected          bool
	client             *ssh.Client
	host               string
	port               int
	user               string
	privateKeyMaterial []byte
}

// Connect establishes a mock SSH connection
func (m *MockSSHClient) Connect() (sshutils.SSHClienter, error) {
	args := m.Called()
	m.connected = true
	return m, args.Error(0)
}

// WaitForSSH implements waiting for SSH connection with retries
func (m *MockSSHClient) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	args := m.Called(ctx, retry, timeout)
	return args.Error(0)
}

// NewSession creates a new mock SSH session
func (m *MockSSHClient) NewSession() (sshutils.SSHSessioner, error) {
	args := m.Called()
	return &MockSSHSession{}, args.Error(1)
}

// GetClient returns the SSH client
func (m *MockSSHClient) GetClient() *ssh.Client {
	return m.client
}

// Configuration getters
func (m *MockSSHClient) GetHost() string               { return m.host }
func (m *MockSSHClient) GetPort() int                  { return m.port }
func (m *MockSSHClient) GetUser() string               { return m.user }
func (m *MockSSHClient) GetPrivateKeyMaterial() []byte { return m.privateKeyMaterial }

// Remote operations
func (m *MockSSHClient) ExecuteCommand(ctx context.Context, command string) (string, error) {
	args := m.Called(ctx, command)
	return args.String(0), args.Error(1)
}

func (m *MockSSHClient) ExecuteCommandWithCallback(
	ctx context.Context,
	command string,
	callback func(string),
) (string, error) {
	args := m.Called(ctx, command, callback)
	return args.String(0), args.Error(1)
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

func (m *MockSSHClient) PushFileWithCallback(
	ctx context.Context,
	remotePath string,
	content []byte,
	executable bool,
	callback func(int64, int64),
) error {
	args := m.Called(ctx, remotePath, content, executable, callback)
	return args.Error(0)
}

// Service management
func (m *MockSSHClient) InstallSystemdService(
	ctx context.Context,
	serviceName, serviceContent string,
) error {
	args := m.Called(ctx, serviceName, serviceContent)
	return args.Error(0)
}

func (m *MockSSHClient) StartService(ctx context.Context, serviceName string) (string, error) {
	args := m.Called(ctx, serviceName)
	return args.String(0), args.Error(1)
}

func (m *MockSSHClient) RestartService(ctx context.Context, serviceName string) (string, error) {
	args := m.Called(ctx, serviceName)
	return args.String(0), args.Error(1)
}

// IsConnected returns the current connection state
func (m *MockSSHClient) IsConnected() bool {
	return m.connected
}

// Close simulates closing the SSH connection
func (m *MockSSHClient) Close() error {
	args := m.Called()
	m.connected = false
	return args.Error(0)
}

// NewMockSSHClient creates a new MockSSHClient instance with default configuration
func NewMockSSHClient() *MockSSHClient {
	return &MockSSHClient{
		connected:          false,
		client:             nil,
		host:               "localhost",
		port:               22,
		user:               "test",
		privateKeyMaterial: []byte("mock-key"),
	}
}
