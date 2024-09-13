package common

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
)

func SetDefaultConfigurations(provider models.DeploymentType) {
	viper.SetDefault("general.project_prefix", "andaime")
	viper.SetDefault("general.log_path", "/var/log/andaime")
	viper.SetDefault("general.log_level", getDefaultLogLevel())
	viper.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("general.ssh_user", "andaime")
	viper.SetDefault("general.ssh_port", 22)

	if provider == models.DeploymentTypeAzure {
		viper.SetDefault("azure.resource_group_name", "andaime-rg")
		viper.SetDefault("azure.resource_group_location", "eastus")
		viper.SetDefault("azure.allowed_ports", globals.DefaultAllowedPorts)
		viper.SetDefault("azure.default_vm_size", "Standard_B2s")
		viper.SetDefault("azure.default_disk_size_gb", globals.DefaultDiskSizeGB)
		viper.SetDefault("azure.default_location", "eastus")
	} else if provider == models.DeploymentTypeGCP {
		viper.SetDefault("gcp.region", "us-central1")
		viper.SetDefault("gcp.zone", "us-central1-a")
		viper.SetDefault("gcp.machine_type", "e2-medium")
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

	deployment.SSHUser = viper.GetString("general.ssh_user")
	deployment.SSHPort = viper.GetInt("general.ssh_port")
	if deployment.SSHPort == 0 {
		deployment.SSHPort = 22
	}

	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %w", err)
	}

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

	orchestratorMachineName := ""
	orchestratorLocation := ""
	orchestratorMessagePrinted := false
	for _, machineConfig := range machineConfigs {
		machine := &models.Machine{
			Location: machineConfig["location"].(string),
			VMSize: viper.GetString(
				fmt.Sprintf("%s.default_machine_type", strings.ToLower(string(provider))),
			),
		}

		if params, ok := machineConfig["parameters"].(map[string]interface{}); ok {
			var count float64
			if count, ok = params["count"].(float64); !ok || count < 1 {
				count = 1
			}

			if orchestrator, ok := params["orchestrator"].(bool); ok &&
				orchestratorMachineName == "" {
				machine.Orchestrator = orchestrator
				orchestratorMachineName = machine.Name
				orchestratorLocation = machine.Location
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
			for i := 0; i < int(count); i++ {
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

	if provider == models.DeploymentTypeAzure {
		deployment.Azure.ResourceGroupName = viper.GetString("azure.resource_group_name")
		deployment.Azure.ResourceGroupLocation = viper.GetString("azure.resource_group_location")
		deployment.AllowedPorts = viper.GetIntSlice("azure.allowed_ports")
		deployment.Azure.DefaultVMSize = viper.GetString("azure.default_vm_size")
		deployment.Azure.DefaultDiskSizeGB = utils.GetSafeDiskSize(
			viper.GetInt("azure.default_disk_size_gb"),
		)
		deployment.Azure.DefaultLocation = viper.GetString("azure.default_location")
	} else if provider == models.DeploymentTypeGCP {
		deployment.GCP.ProjectID = viper.GetString("gcp.project_id")
		deployment.GCP.OrganizationID = viper.GetString("gcp.organization_id")
		deployment.GCP.BillingAccountID = viper.GetString("gcp.billing_account_id")
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
