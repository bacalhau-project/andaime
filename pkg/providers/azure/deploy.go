package azure

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
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

type Deployment struct {
	ResourceGroupName       string
	ResourceGroupLocation   string
	OrchestratorNode        *Machine
	NonOrchestratorMachines []Machine
	VNet                    map[string][]*armnetwork.Subnet
	ProjectID               string
	UniqueID                string
	Tags                    map[string]*string
}

func (d *Deployment) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"ResourceGroupName":       d.ResourceGroupName,
		"ResourceGroupLocation":   d.ResourceGroupLocation,
		"OrchestratorNode":        d.OrchestratorNode,
		"NonOrchestratorMachines": d.NonOrchestratorMachines,
		"VNet":                    d.VNet,
		"ProjectID":               d.ProjectID,
		"UniqueID":                d.UniqueID,
		"Tags":                    d.Tags,
	}
}

// UpdateViperConfig updates the Viper configuration with the current Deployment state
func (d *Deployment) UpdateViperConfig() error {
	v := viper.GetViper()
	deploymentPath := fmt.Sprintf("deployments.azure.%s", d.ResourceGroupName)
	v.Set(deploymentPath, d.ToMap())
	return v.WriteConfig()
}

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(ctx context.Context, disp *display.Display) error {
	l := logger.Get()
	viper := viper.GetViper()

	// Extract Azure-specific configuration
	uniqueID := viper.GetString("azure.unique_id")
	projectID := viper.GetString("general.project_id")

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

	// Ensure tags
	tags := make(map[string]*string)
	tags = EnsureTags(tags, projectID, uniqueID)

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

	resourceGroupName := viper.GetString("azure.resource_prefix") + "-rg-" + time.Now().Format("0601021504")
	if !IsValidResourceGroupName(resourceGroupName) {
		return fmt.Errorf("invali d resource group name: %s", resourceGroupName)
	}

	resourceGroupLocation := viper.GetString("azure.resource_group_location")
	if !internal.IsValidLocation(resourceGroupLocation) {
		return fmt.Errorf("invalid resource group location: %s", resourceGroupLocation)
	}

	// Get or create the resource group
	disp.UpdateStatus(&display.Status{
		ID:     "resource-group",
		Type:   "Azure",
		Status: "Creating",
	})
	resourceGroup, err := p.Client.GetOrCreateResourceGroup(ctx, resourceGroupLocation, resourceGroupName, tags)
	if err != nil {
		disp.UpdateStatus(&display.Status{
			ID:     "resource-group",
			Type:   "Azure",
			Status: "Failed",
		})
		return fmt.Errorf("failed to get or create resource group: %v", err)
	}
	disp.UpdateStatus(&display.Status{
		ID:     "resource-group",
		Type:   "Azure",
		Status: "Created",
	})

	deployment := &Deployment{
		ResourceGroupName:     resourceGroupName,
		ResourceGroupLocation: resourceGroupLocation,
		ProjectID:             projectID,
		UniqueID:              uniqueID,
		Tags:                  tags,
	}

	// Update Viper configuration after creating the deployment
	if err := deployment.UpdateViperConfig(); err != nil {
		return fmt.Errorf("failed to update Viper configuration: %v", err)
	}

	// Get all ports from the viper config
	ports := viper.GetIntSlice("azure.allowed_ports")
	if len(ports) == 0 {
		return fmt.Errorf("no allowed ports found in viper config")
	}
	l.Debugf("Allowed ports: %v", ports)

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

	// For each location, create a virtual network, subnet, and NSG
	var wg sync.WaitGroup
	errChan := make(chan error, len(locations))
	for _, location := range locations {
		wg.Add(1)
		go func(loc string) {
			defer wg.Done()
			disp.UpdateStatus(&display.Status{
				ID:     fmt.Sprintf("vnet-%s", loc),
				Type:   "Azure",
				Status: "Creating",
			})
			vnet, err := p.Client.CreateVirtualNetwork(ctx, *resourceGroup.Name, loc+"-vnet", loc, tags)
			if err != nil {
				errChan <- fmt.Errorf("failed to create virtual network in %s: %v", loc, err)
				disp.UpdateStatus(&display.Status{
					ID:     fmt.Sprintf("vnet-%s", loc),
					Type:   "Azure",
					Status: "Failed",
				})
				return
			}
			disp.UpdateStatus(&display.Status{
				ID:     fmt.Sprintf("vnet-%s", loc),
				Type:   "Azure",
				Status: "Created",
			})

			disp.UpdateStatus(&display.Status{
				ID:     fmt.Sprintf("nsg-%s", loc),
				Type:   "Azure",
				Status: "Creating",
			})
			_, err = p.Client.CreateNetworkSecurityGroup(ctx, *resourceGroup.Name, loc+"-nsg", loc, ports, tags)
			if err != nil {
				errChan <- fmt.Errorf("failed to create network security group in %s: %v", loc, err)
				disp.UpdateStatus(&display.Status{
					ID:     fmt.Sprintf("nsg-%s", loc),
					Type:   "Azure",
					Status: "Failed",
				})
				return
			}
			disp.UpdateStatus(&display.Status{
				ID:     fmt.Sprintf("nsg-%s", loc),
				Type:   "Azure",
				Status: "Created",
			})

			subnets[loc] = []*armnetwork.Subnet{vnet.Properties.Subnets[0]}
			if deployment.VNet == nil {
				deployment.VNet = make(map[string][]*armnetwork.Subnet)
			}
			deployment.VNet[loc] = []*armnetwork.Subnet{vnet.Properties.Subnets[0]}
		}(location)
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update Viper configuration: %v", err)
	}

	// Create public IPs for all machines
	publicIPs := make(map[string]string)
	for _, machine := range append(nonOrchestratorMachines, *orchestratorNode) {
		for i := 0; i < machine.Parameters[0].Count; i++ {
			vmName := fmt.Sprintf("%s-%d", machine.Location, i)
			disp.UpdateStatus(&display.Status{
				ID:     fmt.Sprintf("public-ip-%s", vmName),
				Type:   "Azure",
				Status: "Creating",
			})
			publicIP, err := p.Client.CreatePublicIP(ctx, *resourceGroup.Name,
				vmName+"-ip",
				machine.Location,
				tags)
			if err != nil {
				disp.UpdateStatus(&display.Status{
					ID:     fmt.Sprintf("public-ip-%s", vmName),
					Type:   "Azure",
					Status: "Failed",
				})
				return fmt.Errorf("failed to create public IP for VM %s: %v", vmName, err)
			}
			publicIPs[vmName] = *publicIP.Properties.IPAddress
			disp.UpdateStatus(&display.Status{
				ID:     fmt.Sprintf("public-ip-%s", vmName),
				Type:   "Azure",
				Status: "Created",
			})
		}
	}

	// Process orchestrator node
	if orchestratorNode != nil {
		err := p.processMachines(ctx, []Machine{*orchestratorNode}, resourceGroupName, subnets, publicIPs, projectID, viper, disp)
		if err != nil {
			return fmt.Errorf("failed to process orchestrator node: %v", err)
		}
		deployment.OrchestratorNode = orchestratorNode
		err = deployment.UpdateViperConfig()
		if err != nil {
			return fmt.Errorf("failed to update Viper configuration: %v", err)
		}
	}

	// Process other machines
	for _, machine := range machines {
		err := p.processMachines(ctx, []Machine{machine}, resourceGroupName, subnets, publicIPs, projectID, viper, disp)
		if err != nil {
			return fmt.Errorf("failed to process non-orchestrator nodes: %v", err)
		}
		deployment.NonOrchestratorMachines = append(deployment.NonOrchestratorMachines, machine)
		err = deployment.UpdateViperConfig()
		if err != nil {
			return fmt.Errorf("failed to update Viper configuration: %v", err)
		}
	}

	if resourceGroup != nil && *resourceGroup.Name != "" {
		fmt.Printf("Successfully deployed Azure VM '%s' in resource group '%s'\n", uniqueID, *resourceGroup.Name)
	}
	return nil
}

func (p *AzureProvider) processMachines(
	ctx context.Context,
	machines []Machine,
	resourceGroupName string,
	subnets map[string][]*armnetwork.Subnet,
	publicIPs map[string]string,
	projectID string,
	v *viper.Viper,
	disp *display.Display) error {
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

		var wg sync.WaitGroup
		errChan := make(chan error, internalMachine.Parameters[0].Count)

		for i := 0; i < internalMachine.Parameters[0].Count; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				vmName := fmt.Sprintf("%s-%d", internalMachine.Location, index)
				disp.UpdateStatus(&display.Status{
					ID:     vmName,
					Type:   "Azure",
					Status: "Creating",
				})
				_, err := DeployVM(ctx,
					projectID,
					vmName,
					p.Client,
					v,
					resourceGroupName,
					internalMachine.Location,
					internalMachine.Parameters[0].Type,
					subnets[internalMachine.Location][0],
				)
				if err != nil {
					errChan <- fmt.Errorf("error deploying VM %s in %s: %v", vmName, internalMachine.Location, err)
					disp.UpdateStatus(&display.Status{
						ID:     vmName,
						Type:   "Azure",
						Status: "Failed",
					})
					return
				}
				disp.UpdateStatus(&display.Status{
					ID:        vmName,
					Type:      "Azure",
					Status:    "Created",
					PublicIP:  publicIPs[vmName],
					PrivateIP: "", // You might want to fetch this after VM creation
				})
			}(i)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			if err != nil {
				return err
			}
		}
	}
	return nil
}
