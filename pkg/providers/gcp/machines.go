package gcp

import (
	"context"
	"fmt"

	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

func (p *GCPProvider) ProcessMachinesConfig(
	ctx context.Context,
) (map[string]models.Machiner, map[string]bool, error) {
	return common.ProcessMachinesConfig(models.DeploymentTypeGCP, p.ValidateMachineType)
}

func (p *GCPProvider) ValidateMachineType(
	ctx context.Context,
	location, machineType string,
) (bool, error) {
	if !internal_gcp.IsValidGCPZone(location) {
		return false, fmt.Errorf("invalid location for GCP: %s", location)
	}
	if !internal_gcp.IsValidGCPMachineType(location, machineType) {
		return false, fmt.Errorf("invalid machine type for GCP: %s", machineType)
	}
	return true, nil
}
