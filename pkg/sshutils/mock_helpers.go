package sshutils

import (
	"strings"

	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/stretchr/testify/mock"
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
	mockSSH := new(ssh_mock.MockSSHConfiger)

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
		mockSSH.On("Connect").Return(&ssh_mock.MockSSHClienter{}, nil).Maybe()
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

		if exp.Times > 0 {
			call.Times(exp.Times)
		} else {
			call.Once()
		}

		call.Return(exp.Output, exp.Error)

		if exp.ProgressCallback != nil {
			call.Run(func(args mock.Arguments) {
				exp.ProgressCallback(50, 100)  //nolint:mnd
				exp.ProgressCallback(100, 100) //nolint:mnd
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
				exp.ProgressCallback(50, 100)  //nolint:mnd
				exp.ProgressCallback(100, 100) //nolint:mnd
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
