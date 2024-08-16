package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/spf13/viper"
)

type ServiceState int

const (
	ServiceStateNotStarted ServiceState = iota
	ServiceStateCreated
	ServiceStateUpdating
	ServiceStateSucceeded
	ServiceStateFailed
	ServiceStateUnknown
)

type Machine struct {
	ID            string
	Name          string
	Type          AzureResourceTypes
	Location      string
	StatusMessage string
	Parameters    Parameters
	PublicIP      string
	PrivateIP     string
	StartTime     time.Time

	machineResources map[string]MachineResource

	VMSize       string
	DiskSizeGB   int32 `default:"30"`
	ComputerName string
	ElapsedTime  time.Duration
	Orchestrator bool
	Docker       ServiceState
	CorePackages ServiceState
	Bacalhau     ServiceState
	SSH          ServiceState
}

func (m *Machine) IsOrchestrator() bool {
	return m.Orchestrator
}

func (m *Machine) SSHEnabled() bool {
	return m.SSH == ServiceStateSucceeded
}

func (m *Machine) DockerEnabled() bool {
	return m.Docker == ServiceStateSucceeded
}

func (m *Machine) BacalhauEnabled() bool {
	return m.Bacalhau == ServiceStateSucceeded
}

func (m *Machine) GetResource(resourceType string) MachineResource {
	if m.machineResources == nil {
		m.machineResources = make(map[string]MachineResource)
	}
	if resource, ok := m.machineResources[resourceType]; ok {
		return resource
	}
	return MachineResource{}
}

func (m *Machine) SetResource(resourceType string, resourceState AzureResourceState) {
	if m.machineResources == nil {
		m.machineResources = make(map[string]MachineResource)
	}
	m.machineResources[resourceType] = MachineResource{
		ResourceName:  resourceType,
		ResourceType:  GetAzureResourceType(resourceType),
		ResourceState: resourceState,
		ResourceValue: "",
	}
}

func (m *Machine) ResourcesComplete() (int, int) {
	// Below is the list of resources that are required to be created for a machine
	allResources := []MachineResource{
		m.machineResources[AzureResourceTypeVNET.ResourceString],
		m.machineResources[AzureResourceTypeNIC.ResourceString],
		m.machineResources[AzureResourceTypeNSG.ResourceString],
		m.machineResources[AzureResourceTypeIP.ResourceString],
		m.machineResources[AzureResourceTypeDISK.ResourceString],
		m.machineResources[AzureResourceTypeSNET.ResourceString],
		m.machineResources[AzureResourceTypeVM.ResourceString],
	}

	totalResources := len(allResources)
	completedResources := 0

	for _, resource := range m.machineResources {
		if resource.ResourceState == AzureResourceStateSucceeded {
			completedResources++
		}
	}
	return completedResources, totalResources
}

func (m *Machine) Complete() bool {
	pending, total := m.ResourcesComplete()
	return pending == total &&
		pending > 0 &&
		m.SSH >= ServiceStateSucceeded &&
		m.Docker >= ServiceStateSucceeded &&
		m.Bacalhau >= ServiceStateSucceeded
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
	AzureResourceStateUnknown AzureResourceState = iota
	AzureResourceStateNotStarted
	AzureResourceStatePending
	AzureResourceStateRunning
	AzureResourceStateFailed
	AzureResourceStateSucceeded
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
	ResourceType  AzureResourceTypes
	ResourceState AzureResourceState
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
		Tags: make(map[string]*string),
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

type StatusUpdateMsg struct {
	Status *DisplayStatus
}
