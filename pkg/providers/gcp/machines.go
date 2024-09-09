package gcp

import (
	"fmt"

	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
)

import (
	"context"
	"fmt"

	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

func ProcessMachinesConfig(deployment *models.Deployment) error {
	validateMachineType := func(location, machineType string) (bool, error) {
		if !internal_gcp.IsValidGCPLocation(location) {
			return false, fmt.Errorf("invalid location for GCP: %s", location)
		}
		if !internal_gcp.IsValidGCPMachineType(location, machineType) {
			return false, fmt.Errorf("invalid machine type for GCP: %s", machineType)
		}
		return true, nil
	}

	return common.ProcessMachinesConfig(deployment, models.DeploymentTypeGCP, validateMachineType)
}
