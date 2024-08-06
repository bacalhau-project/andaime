package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	GetClient() AzureClient
	SetClient(client AzureClient)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)

	// SearchResources queries Azure resources based on the provided criteria.
	//
	// It searches for resources within the specified scope and subscription, filtered by tags.
	// The function then converts the raw response into a slice of GenericResource objects.
	//
	// Parameters:
	//   - ctx: The context for the operation, which can be used for cancellation.
	//   - subscriptionID: The ID of the Azure subscription to search within.
	//   - searchScope: The scope within which to search for resources.
	//   - tags: A map of tag keys to pointers of tag values to filter the resources.
	//
	// Returns:
	//   - A slice of pointers to armresources.GenericResource objects representing the found resources.
	//   - An error if the search operation fails or if there's an issue processing the response.
	SearchResources(ctx context.Context,
		subscriptionID, searchScope string,
		tags map[string]*string) ([]*armresources.GenericResource, error)
	DeployResources(ctx context.Context,
		deployment *models.Deployment,
		disp *display.Display) error
	DestroyResources(ctx context.Context,
		resourceGroupName string) error
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

func (p *AzureProvider) DestroyResources(ctx context.Context, resourceGroupName string) error {
	return p.Client.DestroyResourceGroup(ctx, resourceGroupName)
}

func (p *AzureProvider) SearchResources(ctx context.Context,
	subscriptionID string,
	searchScope string,
	tags map[string]*string) ([]*armresources.GenericResource, error) {

	resourcesResponse, err := p.Client.SearchResources(ctx,
		subscriptionID,
		searchScope,
		tags)
	if err != nil {
		logger.Get().Errorf("Failed to query Azure resources: %v", err)
		return nil, fmt.Errorf("failed to query resources: %v", err)
	}

	logger.Get().Debugf("Azure Resource Graph response: %+v", resourcesResponse)

	var resources []*armresources.GenericResource
	for _, data := range resourcesResponse {
		genericResource := &armresources.GenericResource{
			ID:       data.ID,
			Name:     data.Name,
			Type:     data.Type,
			Location: data.Location,
		}
		if properties, ok := data.Properties.(map[string]interface{}); ok {
			if provisioningState, ok := properties["provisioningState"].(string); ok {
				genericResource.Properties = map[string]interface{}{
					"provisioningState": provisioningState,
				}
			}
		}
		resources = append(resources, genericResource)
	}

	return resources, nil
}

var _ AzureProviderer = &AzureProvider{}
