package providers

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

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
