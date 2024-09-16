package providers

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
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
		return azure.NewAzureProvider(ctx)
	case "gcp":
		return gcp.NewGCPProvider(ctx)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}
