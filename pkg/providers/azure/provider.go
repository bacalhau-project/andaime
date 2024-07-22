package azure

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	CreateDeployment(ctx context.Context) error
}

type AzureProvider struct {
	Client AzureClient
	Config *viper.Viper
}

var AzureProviderFunc = NewAzureProvider

// NewAzureProvider creates a new AzureProvider instance
func NewAzureProvider(config *viper.Viper) (AzureProviderer, error) {
	if !config.IsSet("azure") {
		return nil, fmt.Errorf("azure configuration is required")
	}

	if !config.IsSet("azure.subscription_id") {
		return nil, fmt.Errorf("azure.subscription_id is required")
	}

	client, err := NewAzureClientFunc(config.GetString("azure.subscription_id"))
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	return &AzureProvider{
		Client: client,
		Config: config,
	}, nil
}

// CreateDeployment performs the Azure deployment
func (p *AzureProvider) CreateDeployment(ctx context.Context) error {
	return p.DeployResources()
}
