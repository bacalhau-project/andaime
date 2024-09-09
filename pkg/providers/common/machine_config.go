package common

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
)

type RawMachine struct {
	Location   string `yaml:"location"`
	Parameters *struct {
		Count           int    `yaml:"count,omitempty"`
		Type            string `yaml:"type,omitempty"`
		Orchestrator    bool   `yaml:"orchestrator,omitempty"`
		DiskSizeGB      int    `yaml:"disk_size_gb,omitempty"`
		DiskImageURL    string `yaml:"disk_image_url,omitempty"`
		DiskImageFamily string `yaml:"disk_image_family,omitempty"`
	} `yaml:"parameters"`
}

func ProcessMachinesConfig(deployment *models.Deployment, providerType models.DeploymentType, validateMachineType func(string, string) (bool, error)) error {
	locations := make(map[string]bool)

	rawMachines := []RawMachine{}

	// ...

	for _, rawMachine := range rawMachines {
		// ...
		newMachines[newMachine.Name].SetResourceState(
			providerType.String()+"VM",
			models.ResourceStateNotStarted,
		)
	}

	// ...

	// Set orchestrator if not explicitly set
	orchestratorFound := false
	for _, machine := range newMachines {
		if machine.Orchestrator {
			orchestratorFound = true
			break
		}
	}
	if !orchestratorFound && len(newMachines) > 0 {
		// Set the first machine as orchestrator
		for _, machine := range newMachines {
			machine.Orchestrator = true
			break
		}
	}

	deployment.Machines = newMachines
	for k := range locations {
		deployment.Locations = append(deployment.Locations, k)
	}

	return nil
}
