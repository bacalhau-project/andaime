package gcp

import (
	"context"
	"fmt"

	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

func ProcessMachinesConfig() error {
	validateMachineType := func(ctx context.Context, location, machineType string) (bool, error) {
		if !internal_gcp.IsValidGCPLocation(location) {
			return false, fmt.Errorf("invalid location for GCP: %s", location)
		}
		if !internal_gcp.IsValidGCPMachineType(location, machineType) {
			return false, fmt.Errorf("invalid machine type for GCP: %s", machineType)
		}
		return true, nil
	}

	return common.ProcessMachinesConfig(models.DeploymentTypeGCP, validateMachineType)
}
