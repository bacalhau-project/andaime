package providers

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

// NewAzureProvider creates a new Azure provider
func NewAzureProvider(ctx context.Context) (Providerer, error) {
	subscriptionID := viper.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure.subscription_id is required")
	}
	// Implementation details for Azure provider creation
	return nil, fmt.Errorf("Azure provider creation not implemented")
}
