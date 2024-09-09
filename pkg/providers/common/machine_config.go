package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/display"
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

//nolint:funlen,gocyclo
func ProcessMachinesConfig(
	providerType models.DeploymentType,
	validateMachineType func(context.Context, string, string) (bool, error),
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	locations := make(map[string]bool)

	lowerProviderType := strings.ToLower(string(providerType))

	rawMachines := []RawMachine{}
	if err := viper.UnmarshalKey(lowerProviderType+".machines", &rawMachines); err != nil {
		return fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt(string(providerType) + ".default_count_per_zone")
	if defaultCount == 0 {
		errorMessage := fmt.Sprintf("%s.default_count_per_zone is empty", lowerProviderType)
		l.Error(errorMessage)
		return fmt.Errorf(errorMessage)
	}
	defaultType := viper.GetString(string(providerType) + ".default_machine_type")
	if defaultType == "" {
		errorMessage := fmt.Sprintf("%s.default_machine_type is empty", lowerProviderType)
		l.Error(errorMessage)
		return fmt.Errorf(errorMessage)
	}

	defaultDiskSize := viper.GetInt(string(providerType) + ".disk_size_gb")
	if defaultDiskSize == 0 {
		errorMessage := fmt.Sprintf("%s.disk_size_gb is empty", lowerProviderType)
		l.Error(errorMessage)
		return fmt.Errorf(errorMessage)
	}

	privateKeyPath := viper.GetString("general.ssh_private_key_path")
	if privateKeyPath == "" {
		return fmt.Errorf("general.ssh_private_key_path is not set")
	}

	privateKeyBytes, err := sshutils.ReadPrivateKey(privateKeyPath)
	if err != nil {
		errorMessage := fmt.Sprintf("failed to read private key: %v", err)
		l.Error(errorMessage)
		return fmt.Errorf(errorMessage)
	}

	orchestratorIP := m.Deployment.OrchestratorIP
	var orchestratorLocations []string
	for _, rawMachine := range rawMachines {
		if rawMachine.Parameters != nil && rawMachine.Parameters.Orchestrator {
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
		count := utils.GetCountOfMachines(rawMachine.Parameters.Count, defaultCount)
		thisVMType := defaultType
		if rawMachine.Parameters != nil && rawMachine.Parameters.Type != "" {
			thisVMType = rawMachine.Parameters.Type
		}

		fmt.Printf("Validating machine type %s in location %s...", thisVMType, rawMachine.Location)
		valid, err := validateMachineType(context.Background(), rawMachine.Location, thisVMType)
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

		for i := 0; i < count; i++ {
			newMachine, err := createNewMachine(
				providerType,
				rawMachine.Location,
				utils.GetSafeDiskSize(defaultDiskSize),
				thisVMType,
				privateKeyBytes,
				m.Deployment.SSHPort,
			)
			if err != nil {
				return fmt.Errorf("failed to create new machine: %w", err)
			}

			if rawMachine.Parameters != nil {
				if rawMachine.Parameters.Orchestrator {
					newMachine.Orchestrator = true
				}
			} else {
				l.Warnf("Parameters for machine in location %s is nil", rawMachine.Location)
			}

			newMachines[newMachine.Name] = newMachine
			newMachines[newMachine.Name].SetResourceState(
				string(providerType)+"VM",
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

	orchestratorFound := false
	for name, machine := range newMachines {
		if orchestratorIP != "" {
			newMachines[name].OrchestratorIP = orchestratorIP
			orchestratorFound = true
		} else if machine.Orchestrator {
			orchestratorFound = true
		}
	}
	if !orchestratorFound && len(newMachines) > 0 {
		for _, machine := range newMachines {
			machine.Orchestrator = true
			break
		}
		orchestratorFound = true
	}
	if !orchestratorFound {
		return fmt.Errorf("no orchestrator node and orchestratorIP is not set")
	}

	m.Deployment.Machines = newMachines
	for k := range locations {
		m.Deployment.Locations = append(m.Deployment.Locations, k)
	}

	return nil
}

func createNewMachine(
	providerType models.DeploymentType,
	location string,
	diskSizeGB int32,
	vmSize string,
	privateKeyBytes []byte,
	sshPort int,
) (*models.Machine, error) {
	newMachine, err := models.NewMachine(providerType, location, vmSize, diskSizeGB)
	if err != nil {
		return nil, fmt.Errorf("failed to create new machine: %w", err)
	}

	if err := newMachine.EnsureMachineServices(); err != nil {
		logger.Get().Errorf("Failed to ensure machine services: %v", err)
	}

	for _, service := range models.RequiredServices {
		newMachine.SetServiceState(service.Name, models.ServiceStateNotStarted)
	}

	newMachine.SSHUser = "azureuser"
	newMachine.SSHPort = sshPort
	newMachine.SSHPrivateKeyMaterial = privateKeyBytes

	return newMachine, nil
}
