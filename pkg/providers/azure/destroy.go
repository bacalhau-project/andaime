package azure

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

// DestroyAzureDeployment initiates the deletion of the specified Azure resource group
func (p *AzureProvider) DestroyAzureDeployment(ctx context.Context, resourceGroupName string) error {
	l := logger.Get()

	l.Infof("Initiating destruction of Azure deployment (Resource Group: %s)", resourceGroupName)

	err := p.Client.DestroyResourceGroup(ctx, resourceGroupName)
	if err != nil {
		return fmt.Errorf("failed to initiate resource group deletion: %v", err)
	}

	l.Infof("Deletion process for Azure deployment (Resource Group: %s) has been initiated", resourceGroupName)
	return nil
}

func (c *LiveAzureClient) InitiateResourceGroupDeletion(ctx context.Context, resourceGroupName string) error {
	_, err := c.resourceGroupsClient.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		return fmt.Errorf("failed to initiate resource group deletion: %v", err)
	}
	return nil
}

// Ensure AzureProvider implements AzureProviderer interface
var _ AzureProviderer = (*AzureProvider)(nil)
