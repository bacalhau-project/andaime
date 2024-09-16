// pkg/providers/factory.go
package providers

import (
	"context"
	"fmt"

	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp" // Assuming you have a GCP provider
)

// GetProvider returns the appropriate Providerer based on the configuration.
func GetProvider(
	ctx context.Context,
	providerType models.DeploymentType,
) (common.Providerer, error) {
	switch providerType {
	case models.DeploymentTypeAzure:
		subscriptionID := viper.GetString("azure.subscription_id")
		if subscriptionID == "" {
			return nil, fmt.Errorf("azure.subscription_id is not set in configuration")
		}
		return azure.NewAzureProvider(ctx, subscriptionID)
	case models.DeploymentTypeGCP:
		projectID := viper.GetString("gcp.organization_id")
		if projectID == "" {
			return nil, fmt.Errorf("gcp.organization_id is not set in configuration")
		}
		organizationID := viper.GetString("gcp.organization_id")
		if organizationID == "" {
			return nil, fmt.Errorf("gcp.organization_id is not set in configuration")
		}
		return gcp.NewGCPProvider(ctx, projectID, organizationID)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}
