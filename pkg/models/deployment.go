package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/viper"
)

type Machine struct {
	ID                   string
	Name                 string
	Type                 string
	Location             string
	Status               string
	DetailedStatus       string
	Parameters           Parameters
	PublicIP             string
	PrivateIP            string
	StartTime            time.Time
	InstanceID           string
	NetworkSecurityGroup string
	NIC                  string
	VMSize               string
	DiskSizeGB           int32 `default:"30"`
	ComputerName         string
	ElapsedTime          time.Duration
	Orchestrator         bool
	Docker               string
	Bacalhau             string
	SSH                  string
	Progress             int
	ProgressFinish       int
}

type Parameters struct {
	Count        int
	Type         string
	Orchestrator bool
}

type Deployment struct {
	mu                    sync.RWMutex
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
	NetworkWatchers       map[string]*armnetwork.Watcher
	VMExtensionsStatus    map[string]StatusCode
	ProjectID             string
	UniqueID              string
	Tags                  map[string]*string
	AllowedPorts          []int
	SSHPort               int
	SSHPublicKeyPath      string
	SSHPrivateKeyPath     string
	SSHPublicKeyMaterial  string
	SSHPrivateKeyMaterial string
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
	Type      UpdateStatusResourceType
	ID        string
	Status    string
}

func NewDeployment() *Deployment {
	return &Deployment{
		VNet:                  make(map[string]*armnetwork.VirtualNetwork),
		SubnetSlices:          make(map[string][]*armnetwork.Subnet),
		NetworkSecurityGroups: make(map[string]*armnetwork.SecurityGroup),
		Disks:                 make(map[string]*Disk),
		NetworkWatchers:       make(map[string]*armnetwork.Watcher),
		VMExtensionsStatus:    make(map[string]StatusCode),
		Tags:                  make(map[string]*string),
	}
}

func (d *Deployment) ToMap() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
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

func (d *Deployment) UpdateViperConfig() error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v := viper.GetViper()
	deploymentPath := fmt.Sprintf("deployments.azure.%s", d.ResourceGroupName)
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
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.SubnetSlices == nil {
		d.SubnetSlices = make(map[string][]*armnetwork.Subnet)
	}
	d.SubnetSlices[location] = append(d.SubnetSlices[location], subnets...)
}

func (d *Deployment) GetAllResources() ([]AndaimeGenericResource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	resources := []AndaimeGenericResource{}
	for _, vm := range d.Machines {
		resources = append(resources, AndaimeGenericResource{
			MachineID: vm.Name,
			Name:      vm.Name,
			Type:      UpdateStatusResourceTypeVM,
			ID:        vm.InstanceID,
			Status:    vm.Status,
		})
	}
	for _, disk := range d.Disks {
		machineName := strings.TrimSuffix(disk.Name, "-disk")
		resources = append(resources, AndaimeGenericResource{
			MachineID: machineName,
			Name:      disk.Name,
			Type:      UpdateStatusResourceTypeDISK,
			ID:        disk.ID,
			Status:    string(disk.State),
		})
	}
	for _, nsg := range d.NetworkSecurityGroups {
		machineName := strings.TrimSuffix(*nsg.Name, "-nsg")
		resources = append(resources, AndaimeGenericResource{
			MachineID: machineName,
			Name:      *nsg.Name,
			Type:      UpdateStatusResourceTypeNSG,
			ID:        *nsg.ID,
			Status:    string(*nsg.Properties.ProvisioningState),
		})
	}
	for _, vnet := range d.VNet {
		machineName := strings.TrimSuffix(*vnet.Name, "-vnet")
		resources = append(resources, AndaimeGenericResource{
			MachineID: machineName,
			Name:      *vnet.Name,
			Type:      UpdateStatusResourceTypeVNET,
			ID:        *vnet.ID,
			Status:    string(*vnet.Properties.ProvisioningState),
		})
	}
	for _, subnets := range d.SubnetSlices {
		for _, subnet := range subnets {
			machineName := strings.TrimSuffix(*subnet.Name, "-subnet")
			resources = append(resources, AndaimeGenericResource{
				MachineID: machineName,
				Name:      *subnet.Name,
				Type:      UpdateStatusResourceTypeSNET,
				ID:        *subnet.ID,
				Status:    string(*subnet.Properties.ProvisioningState),
			})
		}
	}

	return resources, nil
}

func (d *Deployment) UpdateStatus(status *Status) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, machine := range d.Machines {
		if machine.ID == status.ID {
			d.Machines[i].Status = status.Status
			d.Machines[i].DetailedStatus = status.DetailedStatus
			d.Machines[i].PublicIP = status.PublicIP
			d.Machines[i].PrivateIP = status.PrivateIP
			d.Machines[i].ElapsedTime = status.ElapsedTime
			break
		}
	}
}

type StatusUpdateMsg struct {
	Status *Status
}
