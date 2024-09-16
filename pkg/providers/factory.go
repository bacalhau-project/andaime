package providers

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/viper"
)

// ProviderFactory creates a Provider based on configuration.
func ProviderFactory(ctx context.Context) (Provider, error) {
	providerType := viper.GetString("deployment.provider")
	if providerType == "" {
		return nil, fmt.Errorf("deployment.provider is not set in configuration")
	}

	switch providerType {
	case "azure":
		subscriptionID := viper.GetString("azure.subscription_id")
		if subscriptionID == "" {
			return nil, fmt.Errorf("azure.subscription_id is required")
		}
		azureClient, err := azure.NewAzureClient(subscriptionID)
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure client: %v", err)
		}
		return azure.NewAzureProvider(azureClient), nil
	case "gcp":
		organizationID := viper.GetString("gcp.organization_id")
		if organizationID == "" {
			return nil, fmt.Errorf("gcp.organization_id is required")
		}
		gcpClient, cleanup, err := gcp.NewGCPClient(ctx, organizationID)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCP client: %v", err)
		}
		// You might want to handle cleanup elsewhere if needed
		_ = cleanup
		return gcp.NewGCPProvider(gcpClient), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}
