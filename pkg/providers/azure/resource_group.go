package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// CreateResourceGroup creates a new resource group or returns an existing one
func (c *LiveAzureClient) GetOrCreateResourceGroup(ctx context.Context,
	rgName string,
	rgLocation string,
	tags map[string]string) (*armresources.ResourceGroup, error) {
	log := logger.Get()

	// Get the base resource group name from the config
	if rgName == "" {
		return nil, fmt.Errorf("rgName is not set")
	}

	if !common.IsValidResourceGroupName(rgName) {
		return nil, fmt.Errorf("invalid resource group name: %s", rgName)
	}

	if !internal.IsValidAzureLocation(rgLocation) {
		return nil, fmt.Errorf("invalid resource group location: %s", rgLocation)
	}

	// Check if the resource group already exists
	existing, err := c.GetResourceGroup(ctx, rgLocation, rgName)
	if err == nil {
		return existing, nil
	}

	dereferencedTags := make(map[string]*string)
	for k, v := range tags {
		dereferencedTags[k] = &v
	}

	// Create the resource group
	parameters := armresources.ResourceGroup{
		Name:     to.Ptr(rgName),
		Location: to.Ptr(rgLocation),
		Tags:     dereferencedTags,
	}

	result, err := c.resourceGroupsClient.CreateOrUpdate(ctx, rgName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource group: %v", err)
	}

	log.Infof("Created resource group: %s", rgName)
	return &result.ResourceGroup, nil
}

func (c *LiveAzureClient) GetResourceGroup(ctx context.Context,
	rgLocation string,
	rgName string) (*armresources.ResourceGroup, error) {
	log := logger.Get()

	log.Debugf("Starting Resource Group Get...")
	result, err := c.resourceGroupsClient.Get(ctx, rgName, nil)
	log.Debugf("Resource Group Get result: %v", result)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource group: %v", err)
	}
	rg := result.ResourceGroup
	return &rg, nil
}
