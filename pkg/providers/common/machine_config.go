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
	l := logger.Get()
	locations := make(map[string]bool)

	rawMachines := []RawMachine{}

	if err := viper.UnmarshalKey(fmt.Sprintf("%s.machines", providerType), &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt(fmt.Sprintf("%s.default_count_per_zone", providerType))
	defaultType := viper.GetString(fmt.Sprintf("%s.default_machine_type", providerType))
	defaultDiskSize := viper.GetInt(fmt.Sprintf("%s.disk_size_gb", providerType))

	privateKeyBytes, err := sshutils.ReadPrivateKey(deployment.SSHPrivateKeyPath)
	if err != nil {
		return err
	}

	newMachines := make(map[string]*models.Machine)
	for _, rawMachine := range rawMachines {
		count := defaultCount
		if rawMachine.Parameters != nil && rawMachine.Parameters.Count > 0 {
			count = rawMachine.Parameters.Count
		}

		vmType := defaultType
		if rawMachine.Parameters != nil && rawMachine.Parameters.Type != "" {
			vmType = rawMachine.Parameters.Type
		}

		valid, err := validateMachineType(rawMachine.Location, vmType)
		if !valid || err != nil {
			return fmt.Errorf("invalid machine type %s for location %s: %v", vmType, rawMachine.Location, err)
		}

		diskSizeGB := defaultDiskSize
		if rawMachine.Parameters != nil && rawMachine.Parameters.DiskSizeGB > 0 {
			diskSizeGB = rawMachine.Parameters.DiskSizeGB
		}

		for i := 0; i < count; i++ {
			newMachine, err := CreateNewMachine(
				providerType,
				rawMachine.Location,
				utils.SafeConvertToInt32(diskSizeGB),
				vmType,
				rawMachine.Parameters.DiskImageFamily,
				rawMachine.Parameters.DiskImageURL,
				deployment.SSHPrivateKeyPath,
				privateKeyBytes,
				deployment.SSHPort,
			)
			if err != nil {
				return fmt.Errorf("failed to create new machine: %w", err)
			}

			if rawMachine.Parameters != nil && rawMachine.Parameters.Orchestrator {
				newMachine.Orchestrator = true
			}

			newMachines[newMachine.Name] = newMachine
			newMachines[newMachine.Name].SetResourceState(
				models.GetResourceTypeForProvider(providerType, "VM").ResourceString,
				models.ResourceStateNotStarted,
			)
		}

		locations[rawMachine.Location] = true
	}

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
