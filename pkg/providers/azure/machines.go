package azure

import (
	"context"
	"fmt"

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

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

func ProcessMachinesConfig(deployment *models.Deployment) error {
	azureClient, err := NewAzureClientFunc(deployment.Azure.SubscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create Azure client: %w", err)
	}

	validateMachineType := func(location, machineType string) (bool, error) {
		return azureClient.ValidateMachineType(context.Background(), location, machineType)
	}

	return common.ProcessMachinesConfig(deployment, models.DeploymentTypeAzure, validateMachineType)
}
