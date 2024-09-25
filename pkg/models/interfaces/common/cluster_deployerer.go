package common_interface

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

type ClusterDeployerer interface {
	WaitForAllMachinesToReachState(
		ctx context.Context,
		resourceType string,
		state models.MachineResourceState,
	) error

	ProvisionAllMachinesWithPackages(ctx context.Context) error
	ProvisionPackagesOnMachine(ctx context.Context, machineName string) error
	ExecuteCustomScript(
		ctx context.Context,
		sshConfig sshutils.SSHConfiger,
		machine models.Machiner,
	) error
	ApplyBacalhauConfigs(ctx context.Context, sshConfig sshutils.SSHConfiger) error

	ProvisionBacalhauCluster(ctx context.Context) error
	ProvisionOrchestrator(ctx context.Context, machineName string) error
	ProvisionWorker(ctx context.Context, machineName string) error
}
