package sshutils

import (
	"io"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

type MockSSHClient struct {
	mock.Mock
	WaitForSSHFunc func(host, user, privateKey string) error
	Dialer         SSHDialer
}

func MockSSHClientCreator(
	c SSHClienter,
) func(config *ssh.ClientConfig, dialer SSHDialer) SSHClienter {
	return func(config *ssh.ClientConfig, dialer SSHDialer) SSHClienter { return c }
}

func (m *MockSSHClient) WaitForSSH(host, user, privateKey string) error {
	if m.WaitForSSHFunc != nil {
		return m.WaitForSSHFunc(host, user, privateKey)
	}
	return nil
}

func (m *MockSSHClient) NewSession() (SSHSessioner, error) {
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
