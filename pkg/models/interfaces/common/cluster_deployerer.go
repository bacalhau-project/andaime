package common_interface

import (
	"context"
	"fmt"
	"sort"

	"github.com/bacalhau-project/andaime/pkg/models"
	sshutils_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
)

// Step represents a single step in the provisioning process
type ProvisioningStep string

const (
	SSHConnection       ProvisioningStep = "ssh_connection"
	NodeProvisioning    ProvisioningStep = "node_provisioning"
	NodeConfiguration   ProvisioningStep = "node_configuration"
	BacalhauInstall     ProvisioningStep = "bacalhau_install"
	ServiceScript       ProvisioningStep = "service_script"
	SystemdService      ProvisioningStep = "systemd_service"
	NodeVerification    ProvisioningStep = "node_verification"
	ConfigurationApply  ProvisioningStep = "configuration_apply"
	ServiceRestart      ProvisioningStep = "service_restart"
	RunningCustomScript ProvisioningStep = "running_custom_script"
)

// StepMessage defines the messages and emoji for a provisioning step
type StepMessage struct {
	Emoji        string
	StartMessage string
	DoneMessage  string
	Order        int // Used to maintain the sequence of steps
}

// StepRegistry manages the collection of provisioning steps
type StepRegistry struct {
	steps map[ProvisioningStep]StepMessage
}

// NewStepRegistry creates and initializes a new step registry
func NewStepRegistry() *StepRegistry {
	registry := &StepRegistry{
		steps: make(map[ProvisioningStep]StepMessage),
	}
	registry.initializeDefaultSteps()
	return registry
}

// initializeDefaultSteps sets up the default provisioning steps
func (r *StepRegistry) initializeDefaultSteps() {
	r.RegisterStep(SSHConnection, StepMessage{
		Emoji:        "🔐",
		StartMessage: "Establishing SSH connection...",
		DoneMessage:  "✅ SSH connection established successfully (%s)",
		Order:        1,
	})

	r.RegisterStep(NodeProvisioning, StepMessage{
		Emoji:        "🚦",
		StartMessage: "Starting provisioning for %s (%s)",
		DoneMessage:  "✅ Node initialization complete",
		Order:        2,
	})

	r.RegisterStep(NodeConfiguration, StepMessage{
		Emoji:        "🍴",
		StartMessage: "Setting up node configuration...",
		DoneMessage:  "✅ Node configuration complete",
		Order:        3,
	})
	r.RegisterStep(BacalhauInstall, StepMessage{
		Emoji:        "🐟",
		StartMessage: "Installing Bacalhau...",
		DoneMessage:  "✅ Bacalhau binary installed successfully",
		Order:        4,
	})
	r.RegisterStep(ServiceScript, StepMessage{
		Emoji:        "📝",
		StartMessage: "Installing Bacalhau service script...",
		DoneMessage:  "✅ Bacalhau service script installed",
		Order:        5,
	})
	r.RegisterStep(SystemdService, StepMessage{
		Emoji:        "🔧",
		StartMessage: "Setting up Bacalhau systemd service...",
		DoneMessage:  "✅ Bacalhau systemd service installed and started",
		Order:        6,
	})
	r.RegisterStep(NodeVerification, StepMessage{
		Emoji:        "🔍",
		StartMessage: "Verifying Bacalhau node is running...",
		DoneMessage:  "✅ Bacalhau node verified and running",
		Order:        7,
	})
	r.RegisterStep(ConfigurationApply, StepMessage{
		Emoji:        "📑",
		StartMessage: "Applying %d Bacalhau configurations...",
		DoneMessage:  "✅ Configurations applied successfully",
		Order:        8,
	})
	r.RegisterStep(ServiceRestart, StepMessage{
		Emoji:        "🔄",
		StartMessage: "Restarting service with new configuration...",
		DoneMessage:  "✅ Configuration applied and service restarted",
		Order:        9,
	})
	r.RegisterStep(RunningCustomScript, StepMessage{
		Emoji:        "📜",
		StartMessage: "Running custom configuration script...",
		DoneMessage:  "✅ Custom script executed successfully",
		Order:        10,
	})
}

// RegisterStep adds or updates a step in the registry
func (r *StepRegistry) RegisterStep(step ProvisioningStep, message StepMessage) {
	r.steps[step] = message
}

// GetStep retrieves a step from the registry
func (r *StepRegistry) GetStep(step ProvisioningStep) StepMessage {
	if msg, exists := r.steps[step]; exists {
		return msg
	}
	panic(fmt.Sprintf("step %s not found in registry", step))
}

// GetAllSteps returns all steps in order
func (r *StepRegistry) GetAllSteps() []ProvisioningStep {
	steps := make([]struct {
		step  ProvisioningStep
		order int
	}, 0, len(r.steps))

	for step, msg := range r.steps {
		steps = append(steps, struct {
			step  ProvisioningStep
			order int
		}{step, msg.Order})
	}

	sort.Slice(steps, func(i, j int) bool {
		return steps[i].order < steps[j].order
	})

	result := make([]ProvisioningStep, len(steps))
	for i, s := range steps {
		result[i] = s.step
	}
	return result
}

// TotalSteps returns the total number of registered steps
func (r *StepRegistry) TotalSteps() int {
	return len(r.steps)
}

// StepMessage methods

// RenderStartMessage formats the start message with given arguments
func (s StepMessage) RenderStartMessage(args ...any) string {
	return s.Emoji + " " + fmt.Sprintf(s.StartMessage, args...)
}

// RenderDoneMessage formats the done message with given arguments
func (s StepMessage) RenderDoneMessage(args ...any) string {
	return fmt.Sprintf(s.DoneMessage, args...)
}

type ClusterDeployerer interface {
	ProvisionMachine(
		ctx context.Context,
		sshConfig sshutils_interface.SSHConfiger,
		machine models.Machiner,
	) error
	WaitForAllMachinesToReachState(
		ctx context.Context,
		resourceType string,
		state models.MachineResourceState,
	) error

	ExecuteCustomScript(
		ctx context.Context,
		sshConfig sshutils_interface.SSHConfiger,
		machine models.Machiner,
	) error
	ApplyBacalhauConfigs(ctx context.Context,
		sshConfig sshutils_interface.SSHConfiger,
		bacalhauSettings []models.BacalhauSettings) error

	ProvisionBacalhauCluster(ctx context.Context) error
	ProvisionOrchestrator(ctx context.Context, machineName string) error
	ProvisionWorker(ctx context.Context, machineName string) error
	SetSSHClient(client sshutils_interface.SSHClienter)
}
