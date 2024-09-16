package azure

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// This updates m.Deployment with machines and returns an error if any
func ProcessMachinesConfig(ctx context.Context) error {
	p, err := providers.GetProvider(ctx, models.DeploymentTypeAzure)
	if err != nil {
		return fmt.Errorf("failed to get Azure provider: %w", err)
	}

	validateMachineType := func(ctx context.Context, location, machineType string) (bool, error) {
		return p.ValidateMachineType(ctx, location, machineType)
	}

	return common.ProcessMachinesConfig(models.DeploymentTypeAzure, validateMachineType)
}
