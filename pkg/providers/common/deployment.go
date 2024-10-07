package common

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
)

func isValidScript(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("custom script does not exist: %s", path)
		}
		return fmt.Errorf("error checking custom script: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("custom script is empty: %s", path)
	}
	return nil
}

func SetDefaultConfigurations(provider models.DeploymentType) {
	viper.SetDefault("general.project_prefix", "andaime")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", getDefaultLogLevel())
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("general.ssh_user", DefaultSSHUser)
	viper.SetDefault("general.ssh_port", 22)

	if provider == models.DeploymentTypeAzure {
		viper.SetDefault("azure.default_count_per_zone", 1)
		viper.SetDefault("azure.resource_group_location", "eastus")
		viper.SetDefault("azure.allowed_ports", globals.DefaultAllowedPorts)
		viper.SetDefault("azure.default_machine_type", "Standard_B2s")
		viper.SetDefault("azure.default_disk_size_gb", globals.DefaultDiskSizeGB)
		viper.SetDefault("azure.default_location", "eastus")
	} else if provider == models.DeploymentTypeGCP {
		viper.SetDefault("gcp.default_region", "us-central1")
		viper.SetDefault("gcp.default_zone", "us-central1-a")
		viper.SetDefault("gcp.default_machine_type", "e2-medium")
		viper.SetDefault("gcp.disk_size_gb", globals.DefaultDiskSizeGB)
		viper.SetDefault("gcp.allowed_ports", globals.DefaultAllowedPorts)
	}
}

func PrepareDeployment(
	ctx context.Context,
	provider models.DeploymentType,
) (*models.Deployment, error) {
	l := logger.Get()
	deployment, err := models.NewDeployment()
	if err != nil {
		return nil, fmt.Errorf("failed to create new deployment: %w", err)
	}
	if err := setDeploymentBasicInfo(deployment, provider); err != nil {
		return nil, fmt.Errorf("failed to set deployment basic info: %w", err)
	}

	deployment.DeploymentType = provider
	deployment.StartTime = time.Now()

	deployment.SSHPublicKeyPath,
		deployment.SSHPrivateKeyPath,
		deployment.SSHPublicKeyMaterial,
		deployment.SSHPrivateKeyMaterial,
		err = sshutils.ExtractSSHKeyPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to extract SSH keys: %w", err)
	}

	if err := sshutils.ValidateSSHKeysFromPath(deployment.SSHPublicKeyPath,
		deployment.SSHPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %w", err)
	}

	// Validate the SSH public key format
	if err := sshutils.ValidateSSHPublicKey(deployment.SSHPublicKeyMaterial); err != nil {
		return nil, fmt.Errorf("invalid SSH public key: %w", err)
	}

	deployment.SSHUser = viper.GetString("general.ssh_user")
	deployment.SSHPort = viper.GetInt("general.ssh_port")
	if deployment.SSHPort == 0 {
		deployment.SSHPort = 22
	}

	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %w", err)
	}

	// Check if a custom script is specified and validate it
	customScriptPath := viper.GetString("general.custom_script_path")
	if customScriptPath != "" {
		if err := isValidScript(customScriptPath); err != nil {
			return nil, fmt.Errorf("invalid custom script: %w", err)
		}
		deployment.CustomScriptPath = customScriptPath
	}

	// Validate Bacalhau settings
	bacalhauSettings, err := models.ReadBacalhauSettingsFromViper()
	if err != nil {
		return nil, fmt.Errorf("invalid Bacalhau settings: %w", err)
	}
	deployment.BacalhauSettings = bacalhauSettings

	// Add this after setting provider-specific configurations
	machineConfigsRaw := viper.Get(fmt.Sprintf("%s.machines", strings.ToLower(string(provider))))
	if machineConfigsRaw == nil {
		return nil, fmt.Errorf("no machines configuration found for provider %s", provider)
	}

	var machineConfigs []map[string]interface{}

	switch config := machineConfigsRaw.(type) {
	case []map[string]interface{}:
		machineConfigs = config
	case []interface{}:
		for _, item := range config {
			if machineConfig, ok := item.(map[string]interface{}); ok {
				machineConfigs = append(machineConfigs, machineConfig)
			} else {
				return nil, fmt.Errorf("invalid machine configuration item: expected map[string]interface{}, got %T", item)
			}
		}
	default:
		return nil, fmt.Errorf(
			"invalid machine configuration format: expected []map[string]interface{} or []interface{}, got %T",
			machineConfigsRaw,
		)
	}

	var defaultCountPerZone int
	var defaultDiskSizeGB int

	if provider == models.DeploymentTypeAzure {
		defaultCountPerZone = deployment.Azure.DefaultCountPerZone
		defaultDiskSizeGB = int(deployment.Azure.DefaultDiskSizeGB)
	} else if provider == models.DeploymentTypeGCP {
		defaultCountPerZone = deployment.GCP.DefaultCountPerZone
		defaultDiskSizeGB = int(deployment.GCP.DefaultDiskSizeGB)
	}

	orchestratorMachineName := ""
	orchestratorLocation := ""
	orchestratorMessagePrinted := false
	for _, machineConfig := range machineConfigs {
		machine := &models.Machine{
			Location: machineConfig["location"].(string),
			VMSize: viper.GetString(
				fmt.Sprintf("%s.default_machine_type", strings.ToLower(string(provider))),
			),
			DiskSizeGB:   defaultDiskSizeGB,
			Orchestrator: false,
		}
		machine.SetNodeType(models.BacalhauNodeTypeCompute)

		if params, ok := machineConfig["parameters"].(map[string]interface{}); ok {
			var count int
			switch countValue := params["count"].(type) {
			case int:
				count = countValue
			case string:
				parsedCount, err := strconv.Atoi(countValue)
				if err != nil {
					l.Infof("No count found for machine %s, setting to default: %d", machine.Name,
						defaultCountPerZone)
					parsedCount = defaultCountPerZone
				}
				count = parsedCount
			default:
				countStr := fmt.Sprintf("%v", countValue)
				parsedCount, err := strconv.Atoi(countStr)
				if err != nil {
					l.Infof("No count found for machine %s, setting to default: %d", machine.Name,
						defaultCountPerZone)
					parsedCount = defaultCountPerZone
				}
				count = parsedCount
			}

			if count < 1 {
				count = 1
			}

			if vmSize, ok := params["machine_type"].(string); ok && vmSize != "" {
				machine.VMSize = vmSize
			}

			if diskSizeGB, ok := params["disk_size_gb"].(int); ok && diskSizeGB > 0 {
				machine.DiskSizeGB = diskSizeGB
			}

			if orchestrator, ok := params["orchestrator"].(bool); ok &&
				orchestratorMachineName == "" {
				machine.Orchestrator = orchestrator
				orchestratorMachineName = machine.Name
				orchestratorLocation = machine.Location
				machine.SetNodeType(models.BacalhauNodeTypeOrchestrator)
				if count > 1 && !orchestratorMessagePrinted {
					l.Infof(
						"Orchestrator flag is set, but count is greater than 1. Making the first machine the orchestrator.",
					)
				}
			} else if orchestratorMachineName != "" {
				l.Infof("Orchestrator flag must be set in a single location. Ignoring flag.")
				l.Infof("Orchestrator machine name: %s", orchestratorMachineName)
				l.Infof("Orchestrator location: %s", orchestratorLocation)
			}
			for i := 0; i < count; i++ {
				machine.Name = fmt.Sprintf("%s-vm", utils.GenerateUniqueID())
				deployment.SetMachine(machine.GetName(), machine)
			}
		} else {
			deployment.SetMachine(machine.GetName(), machine)
		}
	}

	return deployment, nil
}

func setDeploymentBasicInfo(deployment *models.Deployment, provider models.DeploymentType) error {
	projectPrefix := viper.GetString("general.project_prefix")
	uniqueID := viper.GetString("general.unique_id")

	if uniqueID == "" {
		uniqueID = time.Now().Format("060102150405")
	}

	deployment.Name = fmt.Sprintf("%s-%s", projectPrefix, uniqueID)
	deployment.SetProjectID(deployment.Name)

	if provider == models.DeploymentTypeAzure {
		deployment.Azure.ResourceGroupLocation = viper.GetString("azure.resource_group_location")
		if deployment.Azure.ResourceGroupLocation == "" {
			return fmt.Errorf("azure.resource_group_location is not set")
		}
		deployment.AllowedPorts = viper.GetIntSlice("azure.allowed_ports")
		deployment.Azure.DefaultVMSize = viper.GetString("azure.default_machine_type")
		if deployment.Azure.DefaultVMSize == "" {
			return fmt.Errorf("azure.default_machine_type is not set")
		}
		deployment.Azure.DefaultDiskSizeGB = utils.GetSafeDiskSize(
			viper.GetInt("azure.default_disk_size_gb"),
		)
		deployment.Azure.DefaultLocation = viper.GetString("azure.default_location")

		deployment.Azure.DefaultCountPerZone = viper.GetInt("azure.default_count_per_zone")
		if deployment.Azure.DefaultCountPerZone == 0 {
			deployment.Azure.DefaultCountPerZone = 1
		}
	} else if provider == models.DeploymentTypeGCP {
		deployment.SetProjectID(deployment.Name)
		deployment.GCP.OrganizationID = viper.GetString("gcp.organization_id")
		if deployment.GCP.OrganizationID == "" {
			return fmt.Errorf("gcp.organization_id is not set")
		}
		deployment.GCP.BillingAccountID = viper.GetString("gcp.billing_account_id")
		if deployment.GCP.BillingAccountID == "" {
			return fmt.Errorf("gcp.billing_account_id is not set")
		}
		deployment.GCP.DefaultRegion = viper.GetString("gcp.default_region")
		if deployment.GCP.DefaultRegion == "" {
			return fmt.Errorf("gcp.default_region is not set")
		}
		deployment.GCP.DefaultZone = viper.GetString("gcp.default_zone")
		if deployment.GCP.DefaultZone == "" {
			return fmt.Errorf("gcp.default_zone is not set")
		}
		deployment.GCP.DefaultMachineType = viper.GetString("gcp.default_machine_type")
		if deployment.GCP.DefaultMachineType == "" {
			return fmt.Errorf("gcp.default_machine_type is not set")
		}
		deployment.GCP.DefaultDiskSizeGB = utils.GetSafeDiskSize(
			viper.GetInt("gcp.default_disk_size_gb"),
		)

		deployment.GCP.DefaultCountPerZone = viper.GetInt("gcp.default_count_per_zone")
		if deployment.GCP.DefaultCountPerZone == 0 {
			deployment.GCP.DefaultCountPerZone = 1
		}
	}

	return nil
}

func getDefaultLogLevel() string {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		return "info"
	}
	return strings.ToLower(logLevel)
}
