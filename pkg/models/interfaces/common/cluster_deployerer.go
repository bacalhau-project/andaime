package common_interface

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

// ProvisioningStep defines a single step in the provisioning process
type ProvisioningStep struct {
	StartMessage  string
	StartProgress int
	DoneMessage   string
	DoneProgress  int
}

// ProvisioningSteps contains all steps in the provisioning process
//
//nolint:gochecknoglobals,mnd
var ProvisioningSteps = struct {
	Start               ProvisioningStep
	SSHConnection       ProvisioningStep
	NodeProvisioning    ProvisioningStep
	BaseSystem          ProvisioningStep
	NodeConfiguration   ProvisioningStep
	BacalhauInstall     ProvisioningStep
	ServiceScript       ProvisioningStep
	SystemdService      ProvisioningStep
	NodeVerification    ProvisioningStep
	RunningCustomScript ProvisioningStep
	Completion          ProvisioningStep
}{
	Start: ProvisioningStep{
		StartMessage:  "🚀 Starting node provisioning process",
		StartProgress: 0,
		DoneMessage:   "",
		DoneProgress:  0,
	},
	SSHConnection: ProvisioningStep{
		StartMessage:  "📡 Establishing SSH connection...",
		StartProgress: 0,
		DoneMessage:   "✅ SSH connection established successfully (%s)",
		DoneProgress:  10,
	},
	BaseSystem: ProvisioningStep{
		StartMessage:  "🏠 Provisioning base packages...",
		StartProgress: 11,
		DoneMessage:   "✅ Base packages provisioned successfully",
		DoneProgress:  20,
	},
	NodeConfiguration: ProvisioningStep{
		StartMessage:  "🍽️ Setting up node configuration...",
		StartProgress: 21,
		DoneMessage:   "✅ Node configuration completed",
		DoneProgress:  30,
	},
	BacalhauInstall: ProvisioningStep{
		StartMessage:  "📦 Installing Bacalhau...",
		StartProgress: 31,
		DoneMessage:   "✅ Bacalhau binary installed successfully",
		DoneProgress:  40,
	},
	ServiceScript: ProvisioningStep{
		StartMessage:  "📝 Installing Bacalhau service script...",
		StartProgress: 41,
		DoneMessage:   "✅ Bacalhau service script installed",
		DoneProgress:  50,
	},
	SystemdService: ProvisioningStep{
		StartMessage:  "🔧 Setting up Bacalhau systemd service...",
		StartProgress: 51,
		DoneMessage:   "✅ Bacalhau systemd service installed and started",
		DoneProgress:  60,
	},
	NodeVerification: ProvisioningStep{
		StartMessage:  "🔍 Verifying Bacalhau node is running...",
		StartProgress: 61,
		DoneMessage:   "✅ Bacalhau node verified and running",
		DoneProgress:  70,
	},
	RunningCustomScript: ProvisioningStep{
		StartMessage:  "🔍 Running custom script...",
		StartProgress: 71,
		DoneMessage:   "✅ Custom script executed successfully",
		DoneProgress:  80,
	},
	Completion: ProvisioningStep{
		StartMessage:  "✅ Node %s successfully provisioned!",
		StartProgress: 71,
		DoneMessage:   "✅ Successfully provisioned node on %s",
		DoneProgress:  80,
	},
}

type ClusterDeployerer interface {
	ProvisionMachine(
		ctx context.Context,
		sshConfig sshutils.SSHConfiger,
		machine models.Machiner,
	) error
	WaitForAllMachinesToReachState(
		ctx context.Context,
		resourceType string,
		state models.MachineResourceState,
	) error

	ExecuteCustomScript(
		ctx context.Context,
		sshConfig sshutils.SSHConfiger,
		machine models.Machiner,
	) error
	ApplyBacalhauConfigs(ctx context.Context,
		sshConfig sshutils.SSHConfiger,
		bacalhauSettings []models.BacalhauSettings) error

	ProvisionBacalhauCluster(ctx context.Context) error
	ProvisionOrchestrator(ctx context.Context, machineName string) error
	ProvisionWorker(ctx context.Context, machineName string) error
}

var _ ClusterDeployerer = &common.ClusterDeployer{}
