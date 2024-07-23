package azure

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	CreateDeployment(ctx context.Context) error
	GetClient() AzureClient
	SetClient(client AzureClient)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
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

	subscriptionID := config.GetString("azure.subscription_id")
	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	return &AzureProvider{
		Client: client,
		Config: config,
	}, nil
}

func (p *AzureProvider) GetClient() AzureClient {
	return p.Client
}

func (p *AzureProvider) SetClient(client AzureClient) {
	p.Client = client
}

func (p *AzureProvider) GetConfig() *viper.Viper {
	return p.Config
}

func (p *AzureProvider) SetConfig(config *viper.Viper) {
	p.Config = config
}

// CreateDeployment performs the Azure deployment
func (p *AzureProvider) CreateDeployment(ctx context.Context) error {
	location := p.Config.GetString("azure.resource_group_location")
	if location == "" {
		return fmt.Errorf("azure.resource_group_location is not set in the configuration")
	}

	rg, err := p.Client.GetOrCreateResourceGroup(ctx, location)
	if err != nil {
		return fmt.Errorf("failed to get or create resource group: %w", err)
	}

	p.Config.Set("azure.resource_group_name", rg.Name)

	return p.DeployResources()
}

var _ AzureProviderer = &AzureProvider{}
