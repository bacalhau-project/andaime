package common_interface

// ProvisioningStep defines a single step in the provisioning process
type ProvisioningStep struct {
	StartMessage  string
	StartProgress int
	DoneMessage   string
	DoneProgress  int
}

// ProvisioningSteps contains all steps in the provisioning process
var ProvisioningSteps = struct {
	Start                 ProvisioningStep
	SSHConnection        ProvisioningStep
	NodeProvisioning     ProvisioningStep
	BaseSystem           ProvisioningStep
	NodeConfiguration    ProvisioningStep
	BacalhauInstall      ProvisioningStep
	ServiceScript        ProvisioningStep
	SystemdService       ProvisioningStep
	NodeVerification     ProvisioningStep
	Completion           ProvisioningStep
}{
	Start: {
		StartMessage:  "🚀 Starting node provisioning process",
		StartProgress: 0,
		DoneMessage:   "",
		DoneProgress:  0,
	},
	SSHConnection: {
		StartMessage:  "📡 Establishing SSH connection...",
		StartProgress: 0,
		DoneMessage:   "✅ SSH connection established successfully",
		DoneProgress:  15,
	},
	NodeProvisioning: {
		StartMessage:  "🔧 Provisioning node on %s",
		StartProgress: 25,
		DoneMessage:   "✅ Node provisioning initiated",
		DoneProgress:  30,
	},
	BaseSystem: {
		StartMessage:  "🏠 Provisioning base system...",
		StartProgress: 35,
		DoneMessage:   "✅ Base system provisioned successfully",
		DoneProgress:  40,
	},
	NodeConfiguration: {
		StartMessage:  "🍽️ Setting up node configuration...",
		StartProgress: 45,
		DoneMessage:   "✅ Node configuration completed",
		DoneProgress:  50,
	},
	BacalhauInstall: {
		StartMessage:  "📦 Installing Bacalhau...",
		StartProgress: 55,
		DoneMessage:   "✅ Bacalhau binary installed successfully",
		DoneProgress:  60,
	},
	ServiceScript: {
		StartMessage:  "📝 Installing Bacalhau service script...",
		StartProgress: 65,
		DoneMessage:   "✅ Bacalhau service script installed",
		DoneProgress:  70,
	},
	SystemdService: {
		StartMessage:  "🔧 Setting up Bacalhau systemd service...",
		StartProgress: 75,
		DoneMessage:   "✅ Bacalhau systemd service installed and started",
		DoneProgress:  80,
	},
	NodeVerification: {
		StartMessage:  "🔍 Verifying Bacalhau node is running...",
		StartProgress: 85,
		DoneMessage:   "✅ Bacalhau node verified and running",
		DoneProgress:  90,
	},
	Completion: {
		StartMessage:  "✅ Node %s successfully provisioned!",
		StartProgress: 95,
		DoneMessage:   "✅ Successfully provisioned node on %s",
		DoneProgress:  100,
	},
}

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

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
