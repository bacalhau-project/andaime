package azure

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
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
func (p *AzureProvider) DeployResources(ctx context.Context) error {
	viper := viper.GetViper()

	// Extract Azure-specific configuration
	uniqueID := viper.GetString("azure.unique_id")

	// Extract SSH public key
	sshPublicKey, err := utils.ExpandPath(viper.GetString("general.ssh_public_key_path"))
	if err != nil {
		return fmt.Errorf("failed to expand path for SSH public key: %v", err)
	}
	sshPrivateKey, err := utils.ExpandPath(viper.GetString("general.ssh_private_key_path"))
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

	var machines []Machine
	err = viper.UnmarshalKey("azure.machines", &machines)
	if err != nil {
		log.Fatalf("Error unmarshaling machines: %v", err)
	}

	projectID := viper.GetString("general.project_id")

	resourceGroupName := viper.GetString("azure.resource_group_prefix") + "-rg-" + time.Now().Format("060102150405")
	if !IsValidResourceGroupName(resourceGroupName) {
		return fmt.Errorf("invalid resource group name: %s", resourceGroupName)
	}

	resourceGroupLocation := viper.GetString("azure.location")
	if !internal.IsValidLocation(resourceGroupLocation) {
		return fmt.Errorf("invalid resource group location: %s", resourceGroupLocation)
	}

	// Get or create the resource group
	resourceGroup, err := p.Client.GetOrCreateResourceGroup(ctx, resourceGroupLocation, resourceGroupName)
	if err != nil {
		return fmt.Errorf("failed to get or create resource group: %v", err)
	}

	// Crawl all of machines and populate locations and which is the orchestrator
	locations := make([]string, 0)
	var orchestratorNode *Machine
	var nonOrchestratorMachines []Machine
	for _, machine := range machines {
		internalMachine := machine
		locations = append(locations, machine.Location)
		if len(internalMachine.Parameters) > 0 && internalMachine.Parameters[0].Orchestrator {
			if orchestratorNode != nil {
				return fmt.Errorf("only one orchestrator node is allowed")
			}
			orchestratorNode = &internalMachine
		} else {
			nonOrchestratorMachines = append(nonOrchestratorMachines, internalMachine)
		}
	}
	// Replace the original machines slice with non-orchestrator machines
	machines = nonOrchestratorMachines

	subnets := make(map[string][]*armnetwork.Subnet)

	// For each location, create a virtual network
	for _, location := range locations {
		vnet, err := p.Client.CreateVirtualNetwork(ctx, *resourceGroup.Name, location+"-vnet", armnetwork.VirtualNetwork{})
		if err != nil {
			return fmt.Errorf("failed to create virtual network: %v", err)
		}
		subnets[location] = vnet.Properties.Subnets
	}

	if orchestratorNode != nil {
		p.processMachines(ctx, []Machine{*orchestratorNode}, subnets, projectID, viper)
	}

	// Process other machines
	for _, machine := range machines {
		p.processMachines(ctx, []Machine{machine}, subnets, projectID, viper)
	}

	if resourceGroup != nil && *resourceGroup.Name != "" {
		fmt.Printf("Successfully deployed Azure VM '%s' in resource group '%s'\n", uniqueID, *resourceGroup.Name)
	}
	return nil
}

func (p *AzureProvider) processMachines(
	ctx context.Context,
	machines []Machine,
	subnets map[string][]*armnetwork.Subnet,
	projectID string,
	v *viper.Viper) {
	defaultCount := v.GetInt("azure.default_count_per_zone")
	defaultType := v.GetString("azure.default_machine_type")

	for _, machine := range machines {
		internalMachine := machine
		count := defaultCount
		machineType := defaultType
		isOrchestrator := false

		if len(internalMachine.Parameters) > 0 {
			// Check if Count is a valid positive integer
			if internalMachine.Parameters[0].Count > 0 {
				count = internalMachine.Parameters[0].Count
			} else {
				count = defaultCount
			}
			if internalMachine.Parameters[0].Type != "" {
				machineType = internalMachine.Parameters[0].Type
			}
			isOrchestrator = internalMachine.Parameters[0].Orchestrator
		}

		internalMachine.Parameters = []Parameters{
			{
				Count:        count,
				Type:         machineType,
				Orchestrator: isOrchestrator,
			},
		}

		for i := 0; i < internalMachine.Parameters[0].Count; i++ {
			err := p.processMachine(ctx,
				&internalMachine,
				subnets,
				projectID,
				v,
			)
			if err != nil {
				log.Printf("Error processing machine in %s: %v", internalMachine.Location, err)
			}
		}
	}
}

// Helper function to process a machine
func (p *AzureProvider) processMachine(
	ctx context.Context,
	machine *Machine,
	subnets map[string][]*armnetwork.Subnet,
	projectID string,
	viper *viper.Viper) error {
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
			Count: viper.GetInt("azure.default_count_per_zone"),
			Type:  viper.GetString("azure.default_machine_type"),
		}
	}

	for i := 0; i < params.Count; i++ {
		uniqueID := fmt.Sprintf("%s-%d", machine.Location, i)
		_, err := DeployVM(ctx,
			projectID,
			uniqueID,
			p.Client,
			viper,
			machine.Location,
			params.Type,
			subnets[machine.Location][0],
		)
		if err != nil {
			log.Printf("Error deploying VM in %s: %v", machine.Location, err)
			return err
		}
	}

	return nil
}
