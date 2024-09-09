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

//nolint:funlen,gocyclo,unused
func ProcessMachinesConfig(deployment *models.Deployment) error {
	l := logger.Get()
	locations := make(map[string]bool)

	rawMachines := []common.RawMachine{}

	if err := viper.UnmarshalKey("azure.machines", &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt("azure.default_count_per_zone")
	if defaultCount == 0 {
		l.Error("azure.default_count_per_zone is empty")
		return fmt.Errorf("azure.default_count_per_zone is empty")
	}
	defaultType := viper.GetString("azure.default_machine_type")
	if defaultType == "" {
		l.Error("azure.default_machine_type is empty")
		return fmt.Errorf("azure.default_machine_type is empty")
	}

	defaultDiskSize := viper.GetInt("azure.disk_size_gb")
	if defaultDiskSize == 0 {
		l.Error("azure.disk_size_gb is empty")
		return fmt.Errorf("azure.disk_size_gb is empty")
	}

	privateKeyBytes, err := sshutils.ReadPrivateKey(deployment.SSHPrivateKeyPath)
	if err != nil {
		return err
	}

	orchestratorIP := deployment.OrchestratorIP
	var orchestratorLocations []string
	for _, rawMachine := range rawMachines {
		if rawMachine.Parameters != nil && rawMachine.Parameters.Orchestrator {
			// We're doing some checking here to make sure that the orchestrator node not
			// specified in a way that would result in multiple orchestrator nodes
			if rawMachine.Parameters.Count == 0 {
				rawMachine.Parameters.Count = defaultCount
			}

			for i := 0; i < rawMachine.Parameters.Count; i++ {
				orchestratorLocations = append(orchestratorLocations, rawMachine.Location)
			}
		}
	}

	if len(orchestratorLocations) > 1 {
		return fmt.Errorf("multiple orchestrator nodes found")
	}

	type badMachineLocationCombo struct {
		location string
		vmSize   string
	}
	var allBadMachineLocationCombos []badMachineLocationCombo
	newMachines := make(map[string]*models.Machine)
	for _, rawMachine := range rawMachines {
		count := 1
		if rawMachine.Parameters != nil {
			if rawMachine.Parameters.Count > 0 {
				count = rawMachine.Parameters.Count
			}
		}

		var thisVMType string
		if rawMachine.Parameters != nil {
			thisVMType = rawMachine.Parameters.Type
			if thisVMType == "" {
				thisVMType = defaultType
			}
		}
		azureClient, err := NewAzureClientFunc(deployment.Azure.SubscriptionID)
		if err != nil {
			return fmt.Errorf("failed to create Azure client: %w", err)
		}

		fmt.Printf("Validating machine type %s in location %s...", thisVMType, rawMachine.Location)
		valid, err := azureClient.ValidateMachineType(
			context.Background(),
			rawMachine.Location,
			thisVMType,
		)
		if !valid || err != nil {
			allBadMachineLocationCombos = append(
				allBadMachineLocationCombos,
				badMachineLocationCombo{
					location: rawMachine.Location,
					vmSize:   thisVMType,
				},
			)
			fmt.Println("❌")
			continue
		}
		fmt.Println("✅")

		countOfMachines := utils.GetCountOfMachines(count, defaultCount)
		for i := 0; i < countOfMachines; i++ {
			newMachine, err := common.CreateNewMachine(
				models.DeploymentTypeAzure,
				rawMachine.Location,
				utils.GetSafeDiskSize(defaultDiskSize),
				thisVMType,
				"", // diskImageFamily
				"", // diskImageURL
				deployment.SSHPrivateKeyPath,
				privateKeyBytes,
				deployment.SSHPort,
			)
			if err != nil {
				return fmt.Errorf("failed to create new machine: %w", err)
			}

			if rawMachine.Parameters != nil {
				if rawMachine.Parameters.Type != "" {
					newMachine.VMSize = rawMachine.Parameters.Type
				}
				if rawMachine.Parameters.Orchestrator {
					newMachine.Orchestrator = true
				}
			} else {
				// Log a warning or handle the case where Parameters is nil
				logger.Get().Warnf("Parameters for machine in location %s is nil", rawMachine.Location)
			}

			newMachines[newMachine.Name] = newMachine
			newMachines[newMachine.Name].SetResourceState(
				models.AzureResourceTypeVM.ResourceString,
				models.ResourceStateNotStarted,
			)
		}

		locations[rawMachine.Location] = true
	}

	if len(allBadMachineLocationCombos) > 0 {
		return fmt.Errorf(
			"invalid machine type and location combinations: %v",
			allBadMachineLocationCombos,
		)
	}

	// Loop for setting the orchestrator node
	orchestratorFound := false
	for name, machine := range newMachines {
		if orchestratorIP != "" {
			newMachines[name].OrchestratorIP = orchestratorIP
			orchestratorFound = true
		} else if machine.Orchestrator {
			orchestratorFound = true
		}
	}
	if !orchestratorFound {
		return fmt.Errorf("no orchestrator node and orchestratorIP is not set")
	}

	deployment.Machines = newMachines
	for k := range locations {
		deployment.Locations = append(deployment.Locations, k)
	}

	return nil
}
