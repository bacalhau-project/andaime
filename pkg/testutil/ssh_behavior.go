// Package testutil provides testing utilities for the Andaime project
package testutil

import (
	"github.com/bacalhau-project/andaime/pkg/models"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

// SSHBehaviorBuilder provides a fluent interface for building SSH mock behaviors
type SSHBehaviorBuilder struct {
	behavior sshutils.ExpectedSSHBehavior
}

// NewSSHBehaviorBuilder creates a new builder for SSH mock behaviors
func NewSSHBehaviorBuilder() *SSHBehaviorBuilder {
	return &SSHBehaviorBuilder{
		behavior: sshutils.ExpectedSSHBehavior{
			WaitForSSHCount: 1,
		},
	}
}

// WithWaitForSSH sets the number of times WaitForSSH should be called
func (b *SSHBehaviorBuilder) WithWaitForSSH(count int) *SSHBehaviorBuilder {
	b.behavior.WaitForSSHCount = count
	return b
}

// WithPushFile adds a file push expectation
func (b *SSHBehaviorBuilder) WithPushFile(dst string, executable bool, times int) *SSHBehaviorBuilder {
	return b.WithPushFileWithCallback(dst, executable, times, nil)
}

// WithPushFileWithCallback adds a file push expectation with progress callback
func (b *SSHBehaviorBuilder) WithPushFileWithCallback(dst string, executable bool, times int, callback func(int64, int64)) *SSHBehaviorBuilder {
	b.behavior.PushFileExpectations = append(b.behavior.PushFileExpectations, sshutils.PushFileExpectation{
		Dst:              dst,
		Executable:       executable,
		Times:           times,
		ProgressCallback: callback,
	})
	return b
}

// WithCommand adds a command execution expectation
func (b *SSHBehaviorBuilder) WithCommand(cmd string, output string, times int) *SSHBehaviorBuilder {
	return b.WithCommandWithCallback(cmd, output, times, nil)
}

// WithCommandWithCallback adds a command execution expectation with progress callback
func (b *SSHBehaviorBuilder) WithCommandWithCallback(cmd string, output string, times int, callback func(int64, int64)) *SSHBehaviorBuilder {
	b.behavior.ExecuteCommandExpectations = append(b.behavior.ExecuteCommandExpectations, sshutils.ExecuteCommandExpectation{
		Cmd:              cmd,
		Output:          output,
		Times:           times,
		ProgressCallback: callback,
	})
	return b
}

// WithSystemdService adds systemd service installation expectation
func (b *SSHBehaviorBuilder) WithSystemdService(times int) *SSHBehaviorBuilder {
	b.behavior.InstallSystemdServiceExpectation = &sshutils.Expectation{
		Times: times,
	}
	return b
}

// WithServiceRestart adds service restart expectation
func (b *SSHBehaviorBuilder) WithServiceRestart(times int) *SSHBehaviorBuilder {
	b.behavior.RestartServiceExpectation = &sshutils.Expectation{
		Times: times,
	}
	return b
}

// WithConnect adds connect expectation
func (b *SSHBehaviorBuilder) WithConnect(client sshutils_interfaces.SSHClienter, times int) *SSHBehaviorBuilder {
	b.behavior.ConnectExpectation = &sshutils.ConnectExpectation{
		Client: client,
		Times:  times,
	}
	return b
}

// WithCustomScript adds custom script expectations
func (b *SSHBehaviorBuilder) WithCustomScript(scriptPath string, times int) *SSHBehaviorBuilder {
	b.WithPushFile("/tmp/custom_script.sh", true, times)
	b.WithCommand("sudo /tmp/custom_script.sh", "", times)
	return b
}

// Build creates a new SSHConfiger with the configured behavior
func (b *SSHBehaviorBuilder) Build() sshutils_interfaces.SSHConfiger {
	return sshutils.NewMockSSHConfigWithBehavior(b.behavior)
}

// Common SSH behavior patterns

// BuildCloudProviderSSHBehavior creates a standard SSH behavior configuration for cloud providers
func BuildCloudProviderSSHBehavior(times int, withCustomScript bool) sshutils.ExpectedSSHBehavior {
	builder := NewSSHBehaviorBuilder().
		WithWaitForSSH(times).
		WithPushFile("/tmp/get-node-config-metadata.sh", true, times).
		WithPushFile("/tmp/install-docker.sh", true, times).
		WithPushFile("/tmp/install-core-packages.sh", true, times).
		WithPushFile("/tmp/install-bacalhau.sh", true, times).
		WithPushFile("/tmp/install-run-bacalhau.sh", true, times).
		WithCommand("sudo /tmp/get-node-config-metadata.sh", "", times).
		WithCommand("sudo /tmp/install-docker.sh", "", times).
		WithCommand(models.ExpectedDockerHelloWorldCommand, models.ExpectedDockerOutput, times).
		WithCommand("sudo /tmp/install-core-packages.sh", "", times).
		WithCommand("sudo /tmp/install-bacalhau.sh", "", times).
		WithCommand("sudo /tmp/install-run-bacalhau.sh", "", times).
		WithSystemdService(times).
		WithServiceRestart(times * 2)

	if withCustomScript {
		builder.WithCustomScript("/tmp/custom_script.sh", times)
	}

	return builder.behavior
}

// BuildDefaultSSHBehavior creates a standard SSH behavior configuration
func BuildDefaultSSHBehavior(times int) sshutils.ExpectedSSHBehavior {
	return sshutils.ExpectedSSHBehavior{
		WaitForSSHCount: times,
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:        "/tmp/get-node-config-metadata.sh",
				Executable: true,
				Times:      times,
			},
			{
				Dst:        "/tmp/install-docker.sh",
				Executable: true,
				Times:      times,
			},
			{
				Dst:        "/tmp/install-core-packages.sh",
				Executable: true,
				Times:      times,
			},
			{
				Dst:        "/tmp/install-bacalhau.sh",
				Executable: true,
				Times:      times,
			},
			{
				Dst:        "/tmp/install-run-bacalhau.sh",
				Executable: true,
				Times:      times,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:   "sudo /tmp/get-node-config-metadata.sh",
				Times: times,
			},
			{
				Cmd:   "sudo /tmp/install-docker.sh",
				Times: times,
			},
			{
				Cmd:    models.ExpectedDockerHelloWorldCommand,
				Output: models.ExpectedDockerOutput,
				Times:  times,
			},
			{
				Cmd:   "sudo /tmp/install-core-packages.sh",
				Times: times,
			},
			{
				Cmd:   "sudo /tmp/install-bacalhau.sh",
				Times: times,
			},
			{
				Cmd:   "sudo /tmp/install-run-bacalhau.sh",
				Times: times,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{Times: times},
		RestartServiceExpectation:        &sshutils.Expectation{Times: times * 2}, // Each service needs restart after install
	}
}
