package common

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
)

type ClusterDeployerer interface {
	WaitForAllMachinesToReachState(
		ctx context.Context,
		resourceType string,
		state models.MachineResourceState,
	) error

	ProvisionAllMachinesWithPackages(ctx context.Context) error
	ProvisionPackagesOnMachine(ctx context.Context, machineName string) error

	ProvisionBacalhauCluster(ctx context.Context) error
	ProvisionOrchestrator(ctx context.Context, machineName string) error
	ProvisionWorker(ctx context.Context, machineName string) error
}