package sshutils

import (
	"context"
	"io"
	"time"

	"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

// MockSSHSessioner implements a mock SSH session
type MockSSHSessioner struct {
	mock.Mock
}

func (m *MockSSHSessioner) Run(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSessioner) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHSessioner) SetStdout(w io.Writer) {
	m.Called(w)
}

func (m *MockSSHSessioner) SetStderr(w io.Writer) {
	m.Called(w)
}

// MockSSHClienter implements a mock SSH client
type MockSSHClienter struct {
	mock.Mock
}

func (m *MockSSHClienter) NewSession() (interfaces.SSHSessioner, error) {
	args := m.Called()
	return args.Get(0).(interfaces.SSHSessioner), args.Error(1)
}

func (m *MockSSHClienter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClienter) GetClient() *ssh.Client {
	args := m.Called()
	return args.Get(0).(*ssh.Client)
}

func (m *MockSSHClienter) Connect() (interfaces.SSHClienter, error) {
	args := m.Called()
	return args.Get(0).(interfaces.SSHClienter), args.Error(1)
}

func (m *MockSSHClienter) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockSSHClientCreator implements a mock SSH client creator
type MockSSHClientCreator struct {
	mock.Mock
}

func (m *MockSSHClientCreator) NewClient(host string, port int, user string, privateKeyPath string, timeout interface{}) (interfaces.SSHClienter, error) {
	args := m.Called(host, port, user, privateKeyPath, timeout)
	return args.Get(0).(interfaces.SSHClienter), args.Error(1)
}

// MockSSHConfiger implements a mock SSH config
type MockSSHConfiger struct {
	mock.Mock
}

func (m *MockSSHConfiger) Connect() (interfaces.SSHClienter, error) {
	args := m.Called()
	return args.Get(0).(interfaces.SSHClienter), args.Error(1)
}

func (m *MockSSHConfiger) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHConfiger) ExecuteCommand(ctx context.Context, cmd string) (string, error) {
	args := m.Called(ctx, cmd)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfiger) ExecuteCommandWithCallback(ctx context.Context, cmd string, callback func(string)) (string, error) {
	args := m.Called(ctx, cmd, callback)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfiger) PushFile(ctx context.Context, dst string, content []byte, executable bool) error {
	args := m.Called(ctx, dst, content, executable)
	return args.Error(0)
}

func (m *MockSSHConfiger) PushFileWithCallback(ctx context.Context, dst string, content []byte, executable bool, callback func(int64, int64)) error {
	args := m.Called(ctx, dst, content, executable, callback)
	return args.Error(0)
}

func (m *MockSSHConfiger) InstallSystemdService(ctx context.Context, name string, content string) error {
	args := m.Called(ctx, name, content)
	return args.Error(0)
}

func (m *MockSSHConfiger) RestartService(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfiger) StartService(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfiger) WaitForSSH(ctx context.Context, retry int, timeout time.Duration) error {
	args := m.Called(ctx, retry, timeout)
	return args.Error(0)
}

func (m *MockSSHConfiger) GetHost() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSSHConfiger) GetPort() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockSSHConfiger) GetUser() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSSHConfiger) GetPrivateKeyMaterial() []byte {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *MockSSHConfiger) GetSSHClient() *ssh.Client {
	args := m.Called()
	return args.Get(0).(*ssh.Client)
}

func (m *MockSSHConfiger) SetSSHClient(client *ssh.Client) {
	m.Called(client)
}

func (m *MockSSHConfiger) SetValidateSSHConnection(fn func() error) {
	m.Called(fn)
}

func (m *MockSSHConfiger) GetSSHClientCreator() interfaces.SSHClientCreator {
	args := m.Called()
	return args.Get(0).(interfaces.SSHClientCreator)
}

func (m *MockSSHConfiger) SetSSHClientCreator(clientCreator interfaces.SSHClientCreator) {
	m.Called(clientCreator)
}

func (m *MockSSHConfiger) GetSFTPClientCreator() interfaces.SFTPClientCreator {
	args := m.Called()
	return args.Get(0).(interfaces.SFTPClientCreator)
}

func (m *MockSSHConfiger) SetSFTPClientCreator(clientCreator interfaces.SFTPClientCreator) {
	m.Called(clientCreator)
}

func (m *MockSSHConfiger) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}
