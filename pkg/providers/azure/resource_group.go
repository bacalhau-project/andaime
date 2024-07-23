package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
)

// CreateResourceGroup creates a new resource group or returns an existing one
func (p *LiveAzureClient) GetOrCreateResourceGroup(ctx context.Context,
	location string) (*armresources.ResourceGroup, error) {
	log := logger.Get()

	// Get the base resource group name from the config
	baseRGName := viper.GetString("azure.resource_group_prefix")
	if baseRGName == "" {
		return nil, fmt.Errorf("azure.resource_group_prefix is not set in the configuration")
	}

	// Append date string to the resource group name
	dateStr := time.Now().Format("0601021504") // YYMMDDHHMM
	rgName := fmt.Sprintf("%s-%s", baseRGName, dateStr)

	// Check if the resource group already exists
	existing, err := p.GetResourceGroup(ctx)
	if err == nil {
		logger.LogAzureAPIEnd("CreateResourceGroup", nil)
		return existing, nil
	}

	// Create the resource group
	parameters := armresources.ResourceGroup{
		Location: to.Ptr(location),
		Tags: map[string]*string{
			"CreatedBy": to.Ptr("Andaime"),
			"CreatedOn": to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}

	result, err := p.resourceGroupsClient.CreateOrUpdate(ctx, rgName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreateResourceGroup", err)
		return nil, fmt.Errorf("failed to create resource group: %v", err)
	}

	// Update Viper config with the new resource group name
	viper.Set("azure.resource_group", rgName)
	if err := viper.WriteConfig(); err != nil {
		log.Warnf("Failed to update config file with new resource group name: %v", err)
	}

	log.Infof("Created resource group: %s", rgName)
	return &result.ResourceGroup, nil
}

func (p *LiveAzureClient) GetResourceGroup(ctx context.Context) (*armresources.ResourceGroup, error) {
	rgName := viper.GetString("azure.resource_group_name")
	if rgName == "" {
		return nil, fmt.Errorf("azure.resource_group_name is not set in the configuration")
	}

	result, err := p.resourceGroupsClient.Get(ctx, rgName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource group: %v", err)
	}
	rg := result.ResourceGroup
	return &rg, nil
}
