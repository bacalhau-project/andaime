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

//nolint:funlen,gocyclo,unused
func ProcessMachinesConfig(deployment *models.Deployment) error {
	l := logger.Get()
	locations := make(map[string]bool)

	rawMachines := []common.RawMachine{}

	if err := viper.UnmarshalKey("gcp.machines", &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt("gcp.default_count_per_zone")
	defaultType := viper.GetString("gcp.default_machine_type")
	defaultDiskSize := viper.GetInt("gcp.disk_size_gb")
	defaultZone := viper.GetString("gcp.default_zone")
	defaultDiskImageFamily := viper.GetString("gcp.default_disk_image_family")
	orgID := viper.GetString("gcp.organization_id")

	if defaultDiskImageFamily == "" {
		defaultDiskImageFamily = "ubuntu-2004-lts"
	}

	if orgID == "" {
		l.Error("gcp.organization_id is empty")
		return fmt.Errorf("gcp.organization_id is empty")
	}

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

		location := rawMachine.Location
		if location == "" {
			location = defaultZone
		}

		diskSizeGB := defaultDiskSize
		if rawMachine.Parameters != nil && rawMachine.Parameters.DiskSizeGB > 0 {
			diskSizeGB = rawMachine.Parameters.DiskSizeGB
		}

		if !internal_gcp.IsValidGCPLocation(location) {
			return fmt.Errorf("invalid location for GCP: %s", location)
		}

		if !internal_gcp.IsValidGCPMachineType(location, vmType) {
			return fmt.Errorf("invalid machine type for GCP: %s", vmType)
		}

		diskImageFamily := defaultDiskImageFamily
		if rawMachine.Parameters != nil && rawMachine.Parameters.DiskImageFamily != "" {
			diskImageFamily = rawMachine.Parameters.DiskImageFamily
		}

		diskImageURL, err := internal_gcp.IsValidGCPDiskImageFamily(location, diskImageFamily)
		if err != nil {
			return fmt.Errorf("invalid disk image family for GCP: %w", err)
		}

		int32DiskSizeGB := utils.SafeConvertToInt32(diskSizeGB)

		for i := 0; i < count; i++ {
			newMachine, err := common.CreateNewMachine(
				models.DeploymentTypeGCP,
				location,
				int32DiskSizeGB,
				vmType,
				diskImageFamily,
				diskImageURL,
				deployment.SSHPrivateKeyPath,
				privateKeyBytes,
				deployment.SSHPort,
			)
			if err != nil {
				return fmt.Errorf("failed to create raw machine: %w", err)
			}

			if rawMachine.Parameters != nil && rawMachine.Parameters.Orchestrator {
				newMachine.Orchestrator = true
			}

			newMachines[newMachine.Name] = newMachine
			newMachines[newMachine.Name].SetResourceState(
				models.GCPResourceTypeInstance.ResourceString,
				models.ResourceStateNotStarted,
			)
		}

		locations[location] = true
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
