package azure

import (
	"context"
	"fmt"
	"log"
	"strings"

	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/utils"
	"github.com/spf13/viper"
)

type Machine struct {
	Location   string
	Parameters []Parameters
}

type Parameters struct {
	Count        int
	Type         string
	Orchestrator bool
}

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources() error {
	ctx := context.Background()
	if p.Config == nil {
		return fmt.Errorf("config is nil")
	}

	config := p.Config

	// Extract Azure-specific configuration
	uniqueID := config.GetString("azure.unique_id")
	resourceGroup := config.GetString("azure.resource_group")

	// Extract SSH public key
	sshPublicKey, err := utils.ExpandPath(config.GetString("general.ssh_public_key_path"))
	if err != nil {
		return fmt.Errorf("failed to expand path for SSH public key: %v", err)
	}
	sshPrivateKey, err := utils.ExpandPath(config.GetString("general.ssh_private_key_path"))
	if err != nil {
		return fmt.Errorf("failed to expand path for SSH private key: %v", err)
	}

	if sshPrivateKey == "" {
		// Then we need to extract the private key from the public key
		sshPrivateKey = strings.TrimSuffix(sshPublicKey, ".pub")
	}

	// Validate SSH keys
	err = sshutils.ValidateSSHKeysFromPath(sshPublicKey, sshPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to validate SSH keys: %v", err)
	}

	v := viper.GetViper()

	var machines []Machine
	err = v.UnmarshalKey("azure.machines", &machines)
	if err != nil {
		log.Fatalf("Error unmarshaling machines: %v", err)
	}

	projectID := v.GetString("general.project_id")
	defaultMachineType := v.GetString("azure.default_machine_type")
	defaultCountPerZone := v.GetInt("azure.default_count_per_zone")

	for _, machine := range machines {
		if !internal.IsValidLocation(machine.Location) {
			log.Fatalf("Error: Invalid location '%s'", machine.Location)
		}

		var params Parameters
		if len(machine.Parameters) > 0 {
			params = machine.Parameters[0]
			if params.Count < 1 {
				log.Fatalf("Error: Count must be at least 1 for location '%s'", machine.Location)
			}
			if !internal.IsValidMachineType(params.Type) {
				log.Fatalf("Error: Invalid machine type '%s' for location '%s'", params.Type, machine.Location)
			}
		} else {
			params = Parameters{
				Count: defaultCountPerZone,
				Type:  defaultMachineType,
			}
		}

		for i := 0; i < params.Count; i++ {
			uniqueID := fmt.Sprintf("%s-%d", machine.Location, i)
			_, err := DeployVM(ctx,
				projectID,
				uniqueID,
				p.Client,
				v,
				machine.Location,
				params.Type,
			)
			if err != nil {
				log.Printf("Error deploying VM in %s: %v", machine.Location, err)
			}
		}
	}
	fmt.Printf("Successfully deployed Azure VM '%s' in resource group '%s'\n", uniqueID, resourceGroup)
	return nil
}
