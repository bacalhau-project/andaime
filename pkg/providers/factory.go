package providers

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/viper"
)

// GetProviderFactory returns the appropriate ProviderFactory based on the configuration
func GetProviderFactory() (ProviderFactory, error) {
	providerType := viper.GetString("deployment.provider")
	if providerType == "" {
		return nil, fmt.Errorf("deployment.provider is not set in configuration")
	}

	switch providerType {
	case "azure":
		return azure.NewAzureProvider, nil
	case "gcp":
		return gcp.NewGCPProvider, nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}
