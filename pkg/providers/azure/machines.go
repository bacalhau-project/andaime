package azure

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// This updates m.Deployment with machines and returns an error if any
func (p *AzureProvider) ProcessMachinesConfig(
	ctx context.Context,
) (map[string]models.Machiner, map[string]bool, error) {
	validateMachineType := func(ctx context.Context, location, machineType string) (bool, error) {
		return p.ValidateMachineType(ctx, location, machineType)
	}

	return common.ProcessMachinesConfig(models.DeploymentTypeAzure, validateMachineType)
}
