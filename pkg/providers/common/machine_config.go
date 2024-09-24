package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
)

//nolint:funlen,gocyclo
func ProcessMachinesConfig(
	providerType models.DeploymentType,
	validateMachineTypeFn func(context.Context, string, string) (bool, error),
) (map[string]models.Machiner, map[string]bool, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	locations := make(map[string]bool)

	lowerProviderType := strings.ToLower(string(providerType))

	rawMachines := []RawMachine{}
	if err := viper.UnmarshalKey(lowerProviderType+".machines", &rawMachines); err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling machines: %w", err)
	}

	defaultCount := viper.GetInt(lowerProviderType + ".default_count_per_zone")
	if defaultCount == 0 {
		errorMessage := fmt.Sprintf("%s.default_count_per_zone is empty", lowerProviderType)
		l.Error(errorMessage)
		return nil, nil, fmt.Errorf(errorMessage)
	}
	defaultType := viper.GetString(lowerProviderType + ".default_machine_type")
	if defaultType == "" {
		errorMessage := fmt.Sprintf("%s.default_machine_type is empty", lowerProviderType)
		l.Error(errorMessage)
		return nil, nil, fmt.Errorf(errorMessage)
	}

	defaultDiskImageFamily := viper.GetString(lowerProviderType + ".default_disk_image_family")
	defaultDiskImageURL := viper.GetString(lowerProviderType + ".default_disk_image_url")
	if ((defaultDiskImageURL == "" && defaultDiskImageFamily == "") ||
		(defaultDiskImageURL != "" && defaultDiskImageFamily != "")) &&
		providerType == models.DeploymentTypeGCP {
		l.Warnf(
			"Neither %s.default_disk_image_url or %s.default_disk_image_family is set. Using Ubuntu 20.04 LTS",
			lowerProviderType,
			lowerProviderType,
		)
	}

	defaultDiskSize := viper.GetInt(string(providerType) + ".default_disk_size_gb")
	if defaultDiskSize == 0 {
		errorMessage := fmt.Sprintf("%s.default_disk_size_gb is empty", lowerProviderType)
		l.Error(errorMessage)
		return nil, nil, fmt.Errorf(errorMessage)
	}

	privateKeyPath := viper.GetString("general.ssh_private_key_path")
	if privateKeyPath == "" {
		return nil, nil, fmt.Errorf("general.ssh_private_key_path is not set")
	}

	privateKeyBytes, err := sshutils.ReadPrivateKey(privateKeyPath)
	if err != nil {
		errorMessage := fmt.Sprintf("failed to read private key: %v", err)
		l.Error(errorMessage)
		return nil, nil, fmt.Errorf(errorMessage)
	}

	sshPort, err := strconv.Atoi(viper.GetString("general.ssh_port"))
	if err != nil {
		l.Warnf("failed to parse ssh_port, using default 22")
		sshPort = 22
	}
	m.Deployment.SSHPort = sshPort

	orchestratorIP := m.Deployment.OrchestratorIP
	var orchestratorLocations []string
	for _, rawMachine := range rawMachines {
		if !rawMachine.Parameters.Orchestrator {
			continue
		}
		if rawMachine.Parameters.Count == 0 {
			rawMachine.Parameters.Count = defaultCount
		}
		for i := 0; i < rawMachine.Parameters.Count; i++ {
			orchestratorLocations = append(orchestratorLocations, rawMachine.Location)
		}
	}

	if len(orchestratorLocations) > 1 {
		return nil, nil, fmt.Errorf("multiple orchestrator nodes found")
	}

	type badMachineLocationCombo struct {
		location string
		vmSize   string
	}
	var allBadMachineLocationCombos []badMachineLocationCombo
	newMachines := make(map[string]models.Machiner)
	for _, rawMachine := range rawMachines {
		count := getCountOfMachines(rawMachine.Parameters, defaultCount)
		thisVMType := defaultType
		if rawMachine.Parameters.Type != "" {
			thisVMType = rawMachine.Parameters.Type
		}

		fmt.Printf("Validating machine type %s in location %s...", thisVMType, rawMachine.Location)
		valid, err := validateMachineTypeFn(context.Background(), rawMachine.Location, thisVMType)
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

		diskImageFamily := defaultDiskImageFamily
		diskImageURL := defaultDiskImageURL
		if rawMachine.Parameters != (RawMachineParams{}) {
			if rawMachine.Parameters.DiskImageFamily != "" {
				diskImageFamily = rawMachine.Parameters.DiskImageFamily
			}
			if rawMachine.Parameters.DiskImageURL != "" {
				diskImageURL = rawMachine.Parameters.DiskImageURL
			}
		}
		for i := 0; i < count; i++ {
			newMachine, err := createNewMachine(
				providerType,
				rawMachine.Location,
				defaultDiskSize,
				thisVMType,
				privateKeyPath,
				privateKeyBytes,
				sshPort,
				diskImageFamily,
				diskImageURL,
			)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create new machine: %w", err)
			}

			if rawMachine.Parameters != (RawMachineParams{}) {
				if rawMachine.Parameters.Orchestrator {
					newMachine.SetOrchestrator(true)
				}
			}
			newMachines[newMachine.GetName()] = newMachine
			newMachines[newMachine.GetName()].SetMachineResourceState(
				string(providerType)+"VM",
				models.ResourceStateNotStarted,
			)
		}

		locations[rawMachine.Location] = true
	}

	if len(allBadMachineLocationCombos) > 0 {
		return nil, nil, fmt.Errorf(
			"invalid machine type and location combinations: %v",
			allBadMachineLocationCombos,
		)
	}

	orchestratorFound := false
	for name, machine := range newMachines {
		if orchestratorIP != "" {
			newMachines[name].SetOrchestratorIP(orchestratorIP)
			orchestratorFound = true
		} else if machine.IsOrchestrator() {
			orchestratorFound = true
		}
	}
	if !orchestratorFound && len(newMachines) > 0 {
		for _, machine := range newMachines {
			machine.SetOrchestrator(true)
			break
		}
		orchestratorFound = true
	}
	if !orchestratorFound {
		return nil, nil, fmt.Errorf("no orchestrator node and orchestratorIP is not set")
	}

	uniqueLocations := make(map[string]bool)
	for k := range locations {
		uniqueLocations[k] = true
	}

	return newMachines, uniqueLocations, nil
}

func createNewMachine(
	providerType models.DeploymentType,
	location string,
	diskSizeGB int,
	vmSize string,
	privateKeyPath string,
	privateKeyBytes []byte,
	sshPort int,
	diskImageFamily string,
	diskImageURL string,
) (models.Machiner, error) {
	l := logger.Get()
	newMachine, err := models.NewMachine(
		providerType,
		location,
		vmSize,
		diskSizeGB,
		models.CloudSpecificInfo{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new machine: %w", err)
	}

	if err := newMachine.EnsureMachineServices(); err != nil {
		logger.Get().Errorf("Failed to ensure machine services: %v", err)
	}

	for _, service := range models.RequiredServices {
		newMachine.SetServiceState(service.Name, models.ServiceStateNotStarted)
	}

	newMachine.SetSSHUser("azureuser")
	newMachine.SetSSHPort(sshPort)
	newMachine.SetSSHPrivateKeyMaterial(privateKeyBytes)
	newMachine.SetSSHPrivateKeyPath(privateKeyPath)

	if providerType == models.DeploymentTypeGCP {
		if diskImageFamily == "" && diskImageURL == "" {
			l.Warnf("Neither disk image family or URL is set, using Ubuntu 20.04 LTS")
			diskImageFamily = "ubuntu-2004-lts"
			diskImageURL = "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-2004-lts"
		} else {
			returnedDiskImageURL, err := internal_gcp.IsValidGCPDiskImageFamily(
				location,
				diskImageFamily,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to validate disk image family: %w", err)
			}
			newMachine.SetDiskImageFamily(diskImageFamily)
			if diskImageURL != returnedDiskImageURL && diskImageURL != "" {
				l.Warnf(
					"disk image URL (%s) does not match, using provided URL: %s",
					returnedDiskImageURL,
					diskImageURL,
				)
			} else if diskImageURL == "" {
				diskImageURL = returnedDiskImageURL
			}
		}
		newMachine.SetDiskImageURL(diskImageURL)
		newMachine.SetDiskImageFamily(diskImageFamily)
	}

	return newMachine, nil
}

func getCountOfMachines(params RawMachineParams, defaultCount int) int {
	if params.Count > 0 {
		return params.Count
	}

	if defaultCount == 0 {
		return 1
	}
	return defaultCount
}
