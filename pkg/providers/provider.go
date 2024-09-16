package providers

import (
	"context"
)

// Providerer is the interface that all cloud providers must implement
type Providerer interface {
	PrepareDeployment(ctx context.Context) (*models.Deployment, error)
	StartResourcePolling(ctx context.Context)
	CreateResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	GetClusterDeployer() *common.ClusterDeployer
}

// ProviderFactory creates a Provider based on configuration.
type ProviderFactory func(ctx context.Context) (Providerer, error)
