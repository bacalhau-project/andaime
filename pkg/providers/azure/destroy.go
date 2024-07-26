package azure

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

// DestroyAzureDeployment deletes the specified Azure resource group
func (p *AzureProvider) DestroyAzureDeployment(ctx context.Context, resourceGroupName string) error {
	l := logger.Get()

	l.Infof("Starting destruction of Azure deployment (Resource Group: %s)", resourceGroupName)

	err := p.Client.DestroyResourceGroup(ctx, resourceGroupName)
	if err != nil {
		return fmt.Errorf("failed to start resource group deletion: %v", err)
	}

	l.Infof("Azure deployment (Resource Group: %s) has been successfully destroyed", resourceGroupName)
	return nil
}

func (c *LiveAzureClient) DestroyResourceGroup(ctx context.Context, resourceGroupName string) error {
	l := logger.Get()

	l.Infof("Starting destruction of Azure deployment (Resource Group: %s)", resourceGroupName)

	poller, err := c.resourceGroupsClient.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		return fmt.Errorf("failed to start resource group deletion: %v", err)
	}

	l.Infof("Waiting for resource group deletion to complete...")

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource group: %v", err)
	}

	l.Infof("Azure deployment (Resource Group: %s) has been successfully destroyed", resourceGroupName)
	return nil
}
