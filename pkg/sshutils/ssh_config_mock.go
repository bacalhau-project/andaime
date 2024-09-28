package sshutils

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/mock"
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
	output := args.String(0)
	m.lastOutput = output
	return output, args.Error(1)
}

func (m *MockSSHConfig) ExecuteCommandWithCallback(
	ctx context.Context,
	cmd string,
	progressCallback func(string),
) (string, error) {
	args := m.Called(ctx, cmd, progressCallback)
	return args.String(0), args.Error(1)
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

func (m *MockSSHConfig) SetSSHClient(sshclient SSHClienter) {
	m.Called(sshclient)
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

var _ SSHConfiger = &MockSSHConfig{}
