package sshutils

import (
	"github.com/stretchr/testify/mock"
)

// ExpectedSSHBehavior holds the expected outcomes for SSH methods
type ExpectedSSHBehavior struct {
	PushFileExpectations             []PushFileExpectation
	ExecuteCommandExpectations       []ExecuteCommandExpectation
	InstallSystemdServiceExpectation *Expectation
	RestartServiceExpectation        *Expectation
	StartServiceExpectation          *Expectation
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

	if behavior.StartServiceExpectation != nil {
		call := mockSSHConfig.On(
			"StartService",
			mock.Anything, // ctx
			mock.Anything, // serviceName
		)
		call.Return(behavior.StartServiceExpectation.Error)
		if behavior.StartServiceExpectation.Times > 0 {
			call.Times(behavior.StartServiceExpectation.Times)
		}
	}

	return mockSSHConfig
}
