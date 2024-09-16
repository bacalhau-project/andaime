package providers

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
)

// Provider defines the common interface for all cloud providers.
type Provider interface {
	// Initialization
	Initialize(ctx context.Context) error

	// Resource Group/Project Management
	GetOrCreateResourceGroup(ctx context.Context) (*models.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context) error

	// VM Management
	CreateVM(ctx context.Context, machine *models.Machine) error
	DeleteVM(ctx context.Context, machine *models.Machine) error
	GetVMExternalIP(ctx context.Context, machine *models.Machine) (string, error)

	// Networking
	SetupNetworking(ctx context.Context) error
	ConfigureFirewall(ctx context.Context, machine *models.Machine) error

	// SKU & Validation
	ValidateMachineType(ctx context.Context, machineType string) (bool, error)

	// Billing (if applicable)
	SetBillingAccount(ctx context.Context, accountID string) error

	// Finalization
	FinalizeDeployment(ctx context.Context) error
}
