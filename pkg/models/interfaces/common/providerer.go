package common

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
)

// Providerer defines the interface that all cloud providers must implement.
type Providerer interface {
	// Internal Tools
	ProcessMachinesConfig(
		ctx context.Context,
	) (map[string]models.Machiner, map[string]bool, error)
	PrepareDeployment(ctx context.Context) (*models.Deployment, error)
	StartResourcePolling(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error

	// Resource Management
	CreateResources(ctx context.Context) error
	DestroyResources(ctx context.Context, deploymentID string) error
	PollResources(ctx context.Context) ([]interface{}, error)
	GetVMExternalIP(
		ctx context.Context,
		vmName string,
		locationData map[string]string,
	) (string, error)

	// Cluster Deployer
	GetClusterDeployer() ClusterDeployerer
	SetClusterDeployer(deployer ClusterDeployerer)

	// Resource Validation
	ValidateMachineType(ctx context.Context, location, machineType string) (bool, error)
}
