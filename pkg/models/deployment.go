package models

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/viper"
)

var (
	MachineStatusInitializing = "Initializing"
	MachineStatusComplete     = "Complete"
	MachineStatusFailed       = "Failed"
)

type Machine struct {
	ID                   string
	Name                 string
	Type                 string
	Location             string
	Status               string
	Parameters           Parameters
	PublicIP             string
	PrivateIP            string
	InstanceID           string
	NetworkSecurityGroup string
	NIC                  string
	VMSize               string
	DiskSizeGB           int32 `default:"30"`
	ComputerName         string
	StartTime            time.Time
}

type Parameters struct {
	Count        int
	Type         string
	Orchestrator bool
}

type Deployment struct {
	Name                  string
	ResourceGroupName     string
	ResourceGroupLocation string
	Locations             []string
	OrchestratorNode      *Machine
	Machines              []Machine
	VNet                  map[string]*armnetwork.VirtualNetwork
	SubnetSlices          map[string][]*armnetwork.Subnet
	NetworkSecurityGroups map[string]*armnetwork.SecurityGroup
	Disks                 map[string]*Disk
	VMExtensionsStatus    map[string]StatusCode
	ProjectID             string
	UniqueID              string
	Tags                  map[string]*string // This needs to be a pointer because that's what Azure requires
	AllowedPorts          []int
	SSHPort               int
	SSHPublicKeyPath      string
	SSHPrivateKeyPath     string
	SSHPublicKeyMaterial  string
	DefaultVMSize         string `default:"Standard_B2s"`
	DefaultDiskSizeGB     int32  `default:"30"`
	DefaultLocation       string `default:"eastus"`
	StartTime             time.Time
	EndTime               time.Time
	SubscriptionID        string
}

type Disk struct {
	Name   string
	ID     string
	SizeGB int32
	State  armcompute.DiskState
}

type AndaimeGenericResource struct {
	MachineID string
	Name      string
	Type      string
	ID        string
	Status    string
}

func (d *Deployment) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"ResourceGroupName":     d.ResourceGroupName,
		"ResourceGroupLocation": d.ResourceGroupLocation,
		"OrchestratorNode":      d.OrchestratorNode,
		"Machines":              d.Machines,
		"VNet":                  d.SubnetSlices,
		"ProjectID":             d.ProjectID,
		"UniqueID":              d.UniqueID,
		"Tags":                  d.Tags,
	}
}

// UpdateViperConfig updates the Viper configuration with the current Deployment state
func (d *Deployment) UpdateViperConfig() error {
	v := viper.GetViper()
	deploymentPath := fmt.Sprintf("deployments.azure.%s", d.ResourceGroupName)
	// Just write machine name, public ip, private ip, and orchestrator bool
	viperMachines := make([]map[string]interface{}, len(d.Machines))
	for i, machine := range d.Machines {
		viperMachines[i] = map[string]interface{}{
			"Name":         machine.Name,
			"PublicIP":     machine.PublicIP,
			"PrivateIP":    machine.PrivateIP,
			"Orchestrator": machine.Parameters.Orchestrator,
		}
	}

	v.Set(deploymentPath, viperMachines)
	return v.WriteConfig()
}

func (d *Deployment) SetSubnet(location string, subnets ...*armnetwork.Subnet) {
	if d.SubnetSlices == nil {
		d.SubnetSlices = make(map[string][]*armnetwork.Subnet)
	}
	d.SubnetSlices[location] = append(d.SubnetSlices[location], subnets...)
}

func (d *Deployment) GetAllResources() ([]AndaimeGenericResource, error) {
	resources := []AndaimeGenericResource{}
	for _, vm := range d.Machines {
		resources = append(resources, AndaimeGenericResource{
			MachineID: vm.ID,
			Name:      vm.Name,
			Type:      "Microsoft.Compute/virtualMachines",
			ID:        vm.InstanceID,
			Status:    vm.Status,
		})
	}
	for _, disk := range d.Disks {
		resources = append(resources, AndaimeGenericResource{
			Name:   disk.Name,
			Type:   "Microsoft.Compute/disks",
			ID:     disk.ID,
			Status: string(disk.State),
		})
	}
	for _, nsg := range d.NetworkSecurityGroups {
		resources = append(resources, AndaimeGenericResource{
			Name:   *nsg.Name,
			Type:   "Microsoft.Network/networkSecurityGroups",
			ID:     *nsg.ID,
			Status: string(*nsg.Properties.ProvisioningState),
		})
	}
	for _, vnet := range d.VNet {
		resources = append(resources, AndaimeGenericResource{
			Name:   *vnet.Name,
			Type:   "Microsoft.Network/virtualNetworks",
			ID:     *vnet.ID,
			Status: string(*vnet.Properties.ProvisioningState),
		})
	}
	for _, subnets := range d.SubnetSlices {
		for _, subnet := range subnets {
			resources = append(resources, AndaimeGenericResource{
				Name:   *subnet.Name,
				Type:   "Microsoft.Network/virtualNetworks/subnets",
				ID:     *subnet.ID,
				Status: string(*subnet.Properties.ProvisioningState),
			})
		}
	}

	return resources, nil
}
