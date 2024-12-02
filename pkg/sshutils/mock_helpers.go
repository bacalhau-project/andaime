package sshutils

import (
	"strings"

	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
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
}

// NewMockSSHConfigWithBehavior creates a new mock SSH config with predefined behavior
func NewMockSSHConfigWithBehavior(behavior ExpectedSSHBehavior) sshutils.SSHConfiger {
	mockSSH := new(ssh_mock.MockSSHConfiger)

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

// Matches checks if the given command string starts with this PrefixCommand.
// The PrefixCommand value acts as the prefix to match against.
//
// Parameters:
//   - cmd: The full command string to check
//
// Returns:
//   - true if cmd starts with this prefix, false otherwise
//
// Example:
//
//	prefix := PrefixCommand("docker")
//	prefix.Matches("docker run") // returns true
//	prefix.Matches("kubectl run") // returns false
func (p PrefixCommand) Matches(cmd string) bool {
	return strings.HasPrefix(cmd, string(p))
}
