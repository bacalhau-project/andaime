package providers

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/viper"
)

// NewGCPProvider creates a new GCP provider
func NewGCPProvider(ctx context.Context) (Providerer, error) {
	organizationID := viper.GetString("gcp.organization_id")
	if organizationID == "" {
		return nil, fmt.Errorf("gcp.organization_id is required")
	}
	client := gcp.NewGCPClient() // Assuming you have a NewGCPClient function
	return gcp.DefaultNewGCPProvider(client), nil
}
