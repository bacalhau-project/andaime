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
func (c *LiveAzureClient) GetOrCreateResourceGroup(ctx context.Context,
	location string) (*armresources.ResourceGroup, error) {
	log := logger.Get()

	// Get the base resource group name from the config
	rgName := viper.GetString("azure.resource_group_name")
	if rgName == "" {
		return nil, fmt.Errorf("azure.resource_group_name is not set in the configuration")
	}

	// Check if the resource group already exists
	existing, err := c.GetResourceGroup(ctx)
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

	result, err := c.resourceGroupsClient.CreateOrUpdate(ctx, rgName, parameters, nil)
	if err != nil {
		logger.LogAzureAPIEnd("CreateResourceGroup", err)
		return nil, fmt.Errorf("failed to create resource group: %v", err)
	}

	// Update Viper config with the new resource group name
	viper.Set("azure.resource_group_name", rgName)
	if err := viper.WriteConfig(); err != nil {
		log.Warnf("Failed to update config file with new resource group name: %v", err)
	}

	log.Infof("Created resource group: %s", rgName)
	return &result.ResourceGroup, nil
}

func (c *LiveAzureClient) GetResourceGroup(ctx context.Context) (*armresources.ResourceGroup, error) {
	rgName := viper.GetString("azure.resource_group_name")
	if rgName == "" {
		return nil, fmt.Errorf("azure.resource_group_name is not set in the configuration")
	}

	result, err := c.resourceGroupsClient.Get(ctx, rgName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource group: %v", err)
	}
	rg := result.ResourceGroup
	return &rg, nil
}
