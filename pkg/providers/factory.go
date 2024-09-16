package providers

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

type Providerer interface {
	Initialize(ctx context.Context) error
	GetOrCreateResourceGroup(ctx context.Context) (*models.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context) error
	CreateVM(ctx context.Context, machine *models.Machine) error
	DeleteVM(ctx context.Context, machine *models.Machine) error
	GetVMExternalIP(ctx context.Context, machine *models.Machine) (string, error)
	SetupNetworking(ctx context.Context) error
	ConfigureFirewall(ctx context.Context, machine *models.Machine) error
	ValidateMachineType(ctx context.Context, machineType string) (bool, error)
	SetBillingAccount(ctx context.Context, accountID string) error
	FinalizeDeployment(ctx context.Context) error
}

// ProviderFactory creates a Provider based on configuration.
func ProviderFactory(ctx context.Context) (Providerer, error) {
	providerType := viper.GetString("deployment.provider")
	if providerType == "" {
		return nil, fmt.Errorf("deployment.provider is not set in configuration")
	}

	switch providerType {
	case "azure":
		return NewAzureProvider(ctx)
	case "gcp":
		return NewGCPProvider(ctx)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// NewAzureProvider creates a new Azure provider
func NewAzureProvider(ctx context.Context) (Providerer, error) {
	subscriptionID := viper.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure.subscription_id is required")
	}
	// Implementation details for Azure provider creation
	return nil, fmt.Errorf("Azure provider creation not implemented")
}

// NewGCPProvider creates a new GCP provider
func NewGCPProvider(ctx context.Context) (Providerer, error) {
	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is required")
	}
	// Implementation details for GCP provider creation
	return nil, fmt.Errorf("GCP provider creation not implemented")
}
