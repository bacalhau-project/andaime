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
	ID         string
	Name       string
	Type       AzureResourceTypes
	Location   string
	Status     string
	Parameters Parameters
	PublicIP   string
	PrivateIP  string
	StartTime  time.Time

	VNet                 MachineResource
	Subnet               MachineResource
	NetworkSecurityGroup MachineResource
	NIC                  MachineResource
	IP                   MachineResource
	Disk                 MachineResource

	VMSize         string
	DiskSizeGB     int32 `default:"30"`
	ComputerName   string
	ElapsedTime    time.Duration
	Orchestrator   bool
	Docker         string
	Bacalhau       string
	SSH            string
	Progress       int
	ProgressFinish int
}

type AzureResourceTypes struct {
	ResourceString    string
	ShortResourceName string
}

var AzureResourceTypeNIC = AzureResourceTypes{
	ResourceString:    "Microsoft.Network/networkInterfaces",
	ShortResourceName: "NIC",
}

var AzureResourceTypeVNET = AzureResourceTypes{
	ResourceString:    "Microsoft.Network/virtualNetworks",
	ShortResourceName: "VNET",
}

var AzureResourceTypeSNET = AzureResourceTypes{
	ResourceString:    "Microsoft.Network/subnets",
	ShortResourceName: "SNET",
}

var AzureResourceTypeNSG = AzureResourceTypes{
	ResourceString:    "Microsoft.Network/networkSecurityGroups",
	ShortResourceName: "NSG",
}

var AzureResourceTypeVM = AzureResourceTypes{
	ResourceString:    "Microsoft.Compute/virtualMachines",
	ShortResourceName: "VM",
}

var AzureResourceTypeDISK = AzureResourceTypes{
	ResourceString:    "Microsoft.Compute/disks",
	ShortResourceName: "DISK",
}

var AzureResourceTypeIP = AzureResourceTypes{
	ResourceString:    "Microsoft.Network/publicIPAddresses",
	ShortResourceName: "IP",
}

func (a *AzureResourceTypes) GetResourceString() string {
	return a.ResourceString
}

func (a *AzureResourceTypes) GetShortResourceName() string {
	return a.ShortResourceName
}

func GetAzureResourceType(resource string) AzureResourceTypes {
	for _, r := range GetAllAzureResources() {
		if strings.EqualFold(r.ResourceString, resource) {
			return r
		}
	}
	return AzureResourceTypes{}
}

func GetAllAzureResources() []AzureResourceTypes {
	return []AzureResourceTypes{
		AzureResourceTypeNIC,
		AzureResourceTypeVNET,
		AzureResourceTypeSNET,
		AzureResourceTypeNSG,
		AzureResourceTypeVM,
		AzureResourceTypeDISK,
		AzureResourceTypeIP,
	}
}

func IsValidResource(resource string) bool {
	return GetAzureResourceType(resource).ResourceString != ""
}

type AzureResourceState int

const (
	AzureResourceStateNotStarted AzureResourceState = iota
	AzureResourceStatePending
	AzureResourceStateRunning
	AzureResourceStateFailed
	AzureResourceStateSucceeded
	AzureResourceStateUnknown
)

func ConvertFromStringToAzureResourceState(s string) AzureResourceState {
	switch s {
	case "Not Started":
		return AzureResourceStateNotStarted
	case "Pending":
		return AzureResourceStatePending
	case "Running":
		return AzureResourceStateRunning
	case "Failed":
		return AzureResourceStateFailed
	case "Succeeded":
		return AzureResourceStateSucceeded
	default:
		return AzureResourceStateUnknown
	}
}

type MachineResource struct {
	ResourceName  string
	ResourceType  string
	ResourceState string
	ResourceValue string
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

type StatusUpdateMsg struct {
	Status *DisplayStatus
}
