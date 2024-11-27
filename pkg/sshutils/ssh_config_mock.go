package sshutils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

// ExpectedSSHBehavior holds the expected outcomes for SSH methods
type ExpectedSSHBehavior struct {
	PushFileExpectations             []PushFileExpectation
	ExecuteCommandExpectations       []ExecuteCommandExpectation
	InstallSystemdServiceExpectation *Expectation
	RestartServiceExpectation        *Expectation
}

type PushFileExpectation struct {
	Dst              string
	FileContents     []byte
	Executable       bool
	ProgressCallback interface{}
	Error            error
	Times            int
}

type ExecuteCommandExpectation struct {
	Cmd              string
	CmdMatcher       func(string) bool
	ProgressCallback interface{}
	Output           interface{}
	Error            error
	Times            int
}

type Expectation struct {
	Error error
	Times int
}

// NewMockSSHConfigWithBehavior creates a mock SSHConfig based on the expected behavior
func NewMockSSHConfigWithBehavior(behavior ExpectedSSHBehavior) *MockSSHConfig {
	mockSSHConfig := new(MockSSHConfig)

	for _, exp := range behavior.PushFileExpectations {
		call := mockSSHConfig.On(
			"PushFile",
			mock.Anything, // ctx
			exp.Dst,
			mock.Anything, // fileContents
			exp.Executable,
		)
		call.Return(exp.Error)
		if exp.Times > 0 {
			call.Times(exp.Times)
		}
	}

	for _, exp := range behavior.ExecuteCommandExpectations {
		var cmdArg interface{}
		if exp.CmdMatcher != nil {
			cmdArg = mock.MatchedBy(exp.CmdMatcher)
		} else {
			cmdArg = exp.Cmd
		}
		call := mockSSHConfig.On(
			"ExecuteCommand",
			mock.Anything, // ctx
			cmdArg,
		)
		call.Return(exp.Output, exp.Error)
		if exp.Times > 0 {
			call.Times(exp.Times)
		}
	}

	if behavior.InstallSystemdServiceExpectation != nil {
		call := mockSSHConfig.On(
			"InstallSystemdService",
			mock.Anything, // ctx
			mock.Anything, // serviceName
			mock.Anything, // serviceContent
		)
		call.Return(behavior.InstallSystemdServiceExpectation.Error)
		if behavior.InstallSystemdServiceExpectation.Times > 0 {
			call.Times(behavior.InstallSystemdServiceExpectation.Times)
		}
	}

	if behavior.RestartServiceExpectation != nil {
		call := mockSSHConfig.On(
			"RestartService",
			mock.Anything, // ctx
			mock.Anything, // serviceName
		)
		call.Return(behavior.RestartServiceExpectation.Error)
		if behavior.RestartServiceExpectation.Times > 0 {
			call.Times(behavior.RestartServiceExpectation.Times)
		}
	}

	return mockSSHConfig
}

// MockSSHConfig is a mock implementation of SSHConfiger
type MockSSHConfig struct {
	mock.Mock
	lastOutput string
}

func (m *MockSSHConfig) PushFile(
	ctx context.Context,
	dst string,
	fileContents []byte,
	executable bool,
) error {
	fmt.Printf("PushFile called with: %s\n", dst)
	args := m.Called(ctx, dst, fileContents, executable)
	return args.Error(0)
}

func (m *MockSSHConfig) PushFileWithCallback(
	ctx context.Context,
	dst string,
	fileContents []byte,
	executable bool,
	progressCallback func(int64, int64),
) error {
	args := m.Called(ctx, dst, fileContents, executable, progressCallback)
	return args.Error(0)
}

func (m *MockSSHConfig) ExecuteCommand(
	ctx context.Context,
	cmd string,
) (string, error) {
	fmt.Printf("ExecuteCommand called with: %s\n", cmd)
	args := m.Called(ctx, cmd)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	// Try to convert the result to a string
	switch v := args.Get(0).(type) {
	case string:
		return v, args.Error(1)
	case []byte:
		return string(v), args.Error(1)
	default:
		// If it's not a string or []byte, convert it to a JSON string
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to convert mock result to string: %v", err)
		}
		return string(jsonBytes), args.Error(1)
	}
}

func (m *MockSSHConfig) ExecuteCommandWithCallback(
	ctx context.Context,
	cmd string,
	progressCallback func(string),
) (string, error) {
	args := m.Called(ctx, cmd, progressCallback)
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

func (m *MockSSHConfig) RestartService(ctx context.Context, serviceName string) error {
	args := m.Called(ctx, serviceName)
	return args.Error(0)
}

func (m *MockSSHConfig) Connect() (SSHClienter, error) {
	args := m.Called()
	return args.Get(0).(SSHClienter), args.Error(1)
}

func (m *MockSSHConfig) Close() error { return nil }

func (m *MockSSHConfig) WaitForSSH(
	ctx context.Context,
	retries int,
	retryDelay time.Duration,
) error {
	args := m.Called(ctx, retries, retryDelay)
	return args.Error(0)
}

func (m *MockSSHConfig) StartService(
	ctx context.Context,
	serviceName string,
) error {
	return nil
}

func (m *MockSSHConfig) GetLastOutput() string {
	return m.lastOutput
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
	return args.Get(0).(SSHDialer)
}

func (m *MockSSHConfig) SetSSHDial(dialer SSHDialer) {
	m.Called(dialer)
}

func (m *MockSSHConfig) SetValidateSSHConnection(callback func() error) {
	m.Called(callback)
}

func (m *MockSSHConfig) GetSSHClienter() SSHClienter {
	args := m.Called()
	return args.Get(0).(SSHClienter)
}

func (m *MockSSHConfig) SetSSHClienter(clienter SSHClienter) {
	m.Called(clienter)
}

func (m *MockSSHConfig) GetSSHClient() *ssh.Client {
	args := m.Called()
	return args.Get(0).(*ssh.Client)
}

func (m *MockSSHConfig) SetSSHClient(client *ssh.Client) {
	m.Called(client)
}

var _ SSHConfiger = &MockSSHConfig{}
