package sshutils

import (
	"context"
	"strings"
	"time"

	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

// ExpectedSSHBehavior defines the expected behavior for the mock
type ExpectedSSHBehavior struct {
	WaitForSSHCount       int
	WaitForSSHError       error
	ExecuteCommandOutputs map[string]struct {
		Output string
		Error  error
	}
	PushFileErrors                   map[string]error
	PushFileExpectations             []PushFileExpectation
	ExecuteCommandExpectations       []ExecuteCommandExpectation
	InstallSystemdServiceExpectation *Expectation
	RestartServiceExpectation        *Expectation
	ConnectExpectation               *ConnectExpectation
}

type ConnectExpectation struct {
	Client sshutils_interfaces.SSHClienter
	Error  error
	Times  int
}

// NewMockSSHConfigWithBehavior creates a new mock SSH config with predefined behavior
func NewMockSSHConfigWithBehavior(behavior ExpectedSSHBehavior) sshutils_interfaces.SSHConfiger {
	mockSSH := &MockSSHConfiger{}

	// Setup Connect behavior
	if behavior.ConnectExpectation != nil {
		call := mockSSH.On("Connect")
		if behavior.ConnectExpectation.Times > 0 {
			call.Times(behavior.ConnectExpectation.Times)
		} else {
			call.Once()
		}
		call.Return(behavior.ConnectExpectation.Client, behavior.ConnectExpectation.Error)
	} else {
		mockSSH.On("Connect").Return(&MockSSHClienter{}, nil).Maybe()
	}

	// Setup Close behavior
	mockSSH.On("Close").Return(nil).Maybe()

	for _, exp := range behavior.ExecuteCommandExpectations {
		call := mockSSH.On(
			"ExecuteCommand",
			mock.Anything,
			mock.MatchedBy(func(cmd string) bool {
				return PrefixCommand(exp.Cmd).Matches(cmd)
			}),
		)

		call.Return(exp.Output, exp.Error)

		if exp.Times > 0 {
			call.Times(exp.Times)
		} else {
			call.Once()
		}

		if exp.ProgressCallback != nil {
			call.Run(func(args mock.Arguments) {
				exp.ProgressCallback(50, 100)
				exp.ProgressCallback(100, 100)
			})
		}
	}

	// Setup default ExecuteCommand behavior for unmatched calls
	mockSSH.On("ExecuteCommand",
		mock.Anything,
		mock.Anything,
	).Return("", nil).Maybe()

	// Setup PushFile expectations
	for _, exp := range behavior.PushFileExpectations {
		call := mockSSH.On("PushFile",
			mock.Anything,
			exp.Dst,
			mock.Anything,
			mock.Anything,
		)

		if exp.Times > 0 {
			call.Times(exp.Times)
		} else {
			call.Once()
		}

		call.Return(exp.Error)

		if exp.ProgressCallback != nil {
			call.Run(func(args mock.Arguments) {
				exp.ProgressCallback(50, 100)
				exp.ProgressCallback(100, 100)
			})
		}
	}

	// Setup default PushFile behavior
	mockSSH.On("PushFile",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil).Maybe()

	// Setup InstallSystemdService behavior
	if behavior.InstallSystemdServiceExpectation != nil {
		call := mockSSH.On("InstallSystemdService",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		)

		if behavior.InstallSystemdServiceExpectation.Times > 0 {
			call.Times(behavior.InstallSystemdServiceExpectation.Times)
		}

		call.Return(behavior.InstallSystemdServiceExpectation.Error)
	} else {
		mockSSH.On("InstallSystemdService",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil).Maybe()
	}

	// Setup RestartService behavior
	if behavior.RestartServiceExpectation != nil {
		call := mockSSH.On("RestartService",
			mock.Anything,
			mock.Anything,
		)

		if behavior.RestartServiceExpectation.Times > 0 {
			call.Times(behavior.RestartServiceExpectation.Times)
		}

		call.Return("", behavior.RestartServiceExpectation.Error)
	} else {
		mockSSH.On("RestartService",
			mock.Anything,
			mock.Anything,
		).Return("", nil).Maybe()
	}

	// Setup WaitForSSH behavior
	mockSSH.On("WaitForSSH",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(behavior.WaitForSSHError).Maybe()

	return mockSSH
}

type PushFileExpectation struct {
	Dst              string
	Executable       bool
	ProgressCallback func(int64, int64)
	Error            error
	Times            int
}

type ExecuteCommandExpectation struct {
	Cmd              string
	ProgressCallback func(int64, int64)
	Output           string
	Error            error
	Times            int
}

type Expectation struct {
	Error error
	Times int
}

// CommandMatcher is an interface for matching command strings
type CommandMatcher interface {
	Matches(cmd string) bool
}

// StringCommand is for exact matching
type StringCommand string

func (s StringCommand) Matches(cmd string) bool {
	return string(s) == cmd
}

// PrefixCommand is a command matcher that checks if a command starts with a specific prefix
type PrefixCommand string

func (p PrefixCommand) Matches(cmd string) bool {
	return strings.HasPrefix(cmd, string(p))
}

// Mock types for SSH interfaces
type MockSSHConfiger struct {
	mock.Mock
}

func (m *MockSSHConfiger) Connect() (sshutils_interfaces.SSHClienter, error) {
	args := m.Called()
	return args.Get(0).(sshutils_interfaces.SSHClienter), args.Error(1)
}

func (m *MockSSHConfiger) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHConfiger) ExecuteCommand(ctx context.Context, cmd string) (string, error) {
	args := m.Called(ctx, cmd)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfiger) ExecuteCommandWithCallback(
	ctx context.Context,
	cmd string,
	callback func(string),
) (string, error) {
	args := m.Called(ctx, cmd, callback)
	return args.String(0), args.Error(1)
}

func (m *MockSSHConfiger) PushFile(
	ctx context.Context,
	dst string,
	content []byte,
	executable bool,
) error {
	args := m.Called(ctx, dst, content, executable)
	return args.Error(0)
}

func (m *MockSSHConfiger) PushFileWithCallback(
	ctx context.Context,
	dst string,
	content []byte,
	executable bool,
	callback func(int64, int64),
) error {
	args := m.Called(ctx, dst, content, executable, callback)
	return args.Error(0)
}

func (m *MockSSHConfiger) InstallSystemdService(
	ctx context.Context,
	name string,
	content string,
) error {
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

func (m *MockSSHConfiger) GetSSHClientCreator() sshutils_interfaces.SSHClientCreator {
	args := m.Called()
	return args.Get(0).(sshutils_interfaces.SSHClientCreator)
}

func (m *MockSSHConfiger) SetSSHClientCreator(clientCreator sshutils_interfaces.SSHClientCreator) {
	m.Called(clientCreator)
}

func (m *MockSSHConfiger) GetSFTPClientCreator() sshutils_interfaces.SFTPClientCreator {
	args := m.Called()
	return args.Get(0).(sshutils_interfaces.SFTPClientCreator)
}

func (m *MockSSHConfiger) SetSFTPClientCreator(
	clientCreator sshutils_interfaces.SFTPClientCreator,
) {
	m.Called(clientCreator)
}

func (m *MockSSHConfiger) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockSSHClienter implements a mock SSH client
type MockSSHClienter struct {
	mock.Mock
}

func (m *MockSSHClienter) NewSession() (sshutils_interfaces.SSHSessioner, error) {
	args := m.Called()
	return args.Get(0).(sshutils_interfaces.SSHSessioner), args.Error(1)
}

func (m *MockSSHClienter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHClienter) GetClient() *ssh.Client {
	args := m.Called()
	return args.Get(0).(*ssh.Client)
}

func (m *MockSSHClienter) Connect() (sshutils_interfaces.SSHClienter, error) {
	args := m.Called()
	return args.Get(0).(sshutils_interfaces.SSHClienter), args.Error(1)
}

func (m *MockSSHClienter) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}
