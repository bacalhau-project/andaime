package sshutils

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/stretchr/testify/mock"
)

type MockSSHClient struct {
	mock.Mock
	Session SSHSessioner
	Dialer  SSHDialer
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
	if m.Session != nil {
		return m.Session, nil
	}
	args := m.Called()
	return args.Get(0).(SSHSessioner), args.Error(1)
}

func (m *MockSSHClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func GetTypedMockClient(t *testing.T, log *logger.Logger) (*MockSSHClient, SSHConfiger) {
	mockSSHClient, sshConfig := NewMockSSHClient(NewMockSSHDialer())
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
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSSHSession) StdinPipe() (io.WriteCloser, error) {
	args := m.Called()
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (m *MockSSHSession) StdoutPipe() (io.Reader, error) {
	args := m.Called()
	return args.Get(0).(io.Reader), args.Error(1)
}

func (m *MockSSHSession) StderrPipe() (io.Reader, error) {
	args := m.Called()
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

// MockSSHConfig is a mock implementation of sshutils.SSHConfiger
type MockSSHConfig struct {
	mock.Mock
	Name        string
	MockClient  *MockSSHClient
	MockSession *MockSSHSession
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

func (m *MockSSHConfig) ExecuteCommand(
	ctx context.Context,
	cmd string,
) (string, error) {
	args := m.Called(ctx, cmd)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockSSHConfig) InstallSystemdService(
	ctx context.Context,
	serviceName string,
	serviceContent string,
) error {
	args := m.Called(ctx, serviceName, serviceContent)
	return args.Error(0)
}

func (m *MockSSHConfig) RestartService(
	ctx context.Context,
	serviceName string,
) error {
	args := m.Called(ctx, serviceName)
	return args.Error(0)
}

// Additional methods to satisfy the SSHConfiger interface
func (m *MockSSHConfig) Connect() (SSHClienter, error) { return m.MockClient, nil }

func (m *MockSSHConfig) Close() error { return nil }

func (m *MockSSHConfig) WaitForSSH(
	ctx context.Context,
	retries int,
	retryDelay time.Duration,
) error {
	args := m.Called(ctx, retries, retryDelay)
	return args.Error(0)
}
func (m *MockSSHConfig) SetSSHClient(client SSHClienter) {}

func (m *MockSSHConfig) StartService(
	ctx context.Context,
	serviceName string,
) error {
	return nil
}
