package azure

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// This updates m.Deployment with machines and returns an error if any
func ProcessMachinesConfig() error {
	m := display.GetGlobalModelFunc()
	azureClient, err := NewAzureClientFunc(m.Deployment.Azure.SubscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create Azure client: %w", err)
	}

	validateMachineType := func(ctx context.Context, location, machineType string) (bool, error) {
		return azureClient.ValidateMachineType(ctx, location, machineType)
	}

	return common.ProcessMachinesConfig(models.DeploymentTypeAzure, validateMachineType)
}
