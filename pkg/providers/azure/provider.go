package azure

import (
	"context"
	"fmt"
	"strings"

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

	DeployResources(ctx context.Context, deployment *models.Deployment, disp *display.Display) error
	DestroyResources(ctx context.Context, resourceGroupName string) error
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

func (p *AzureProvider) SearchResources(ctx context.Context, searchScope string, subscriptionID string, tags map[string]string) ([]*armresources.GenericResource, error) {
	var tagFilters []string
	for key, value := range tags {
		tagFilters = append(tagFilters, fmt.Sprintf("tags['%s'] == '%s'", key, value))
	}
	tagFilterString := strings.Join(tagFilters, " and ")

	query := fmt.Sprintf("Resources | where %s | project id, name, type, location, properties.provisioningState", tagFilterString)

	logger.Get().Debugf("Azure Resource Graph query: %s", query)

	resourcesResponse, err := p.Client.Resources(ctx, query, nil)
	if err != nil {
		logger.Get().Errorf("Failed to query Azure resources: %v", err)
		return nil, fmt.Errorf("failed to query resources: %v", err)
	}

	logger.Get().Debugf("Azure Resource Graph response: %+v", resourcesResponse)

	var resources []*armresources.GenericResource
	for _, data := range resourcesResponse.Data {
		if resource, ok := data.(map[string]interface{}); ok {
			genericResource := &armresources.GenericResource{
				ID:       (*string)(resource["id"].(*string)),
				Name:     (*string)(resource["name"].(*string)),
				Type:     (*string)(resource["type"].(*string)),
				Location: (*string)(resource["location"].(*string)),
			}
			if provisioningState, ok := resource["properties"].(map[string]interface{})["provisioningState"].(string); ok {
				genericResource.Properties = map[string]interface{}{
					"provisioningState": provisioningState,
				}
			}
			resources = append(resources, genericResource)
		}
	}

	return resources, nil
}

var _ AzureProviderer = &AzureProvider{}
