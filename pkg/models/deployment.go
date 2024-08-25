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

type ServiceType struct {
	Name  string
	State ServiceState
}

var (
	ServiceTypeSSH      = ServiceType{Name: "SSH", State: ServiceStateNotStarted}
	ServiceTypeDocker   = ServiceType{Name: "Docker", State: ServiceStateNotStarted}
	ServiceTypeBacalhau = ServiceType{Name: "Bacalhau", State: ServiceStateNotStarted}
)

var RequiredAzureResources = []AzureResourceTypes{
	AzureResourceTypeVNET,
	AzureResourceTypeNIC,
	AzureResourceTypeNSG,
	AzureResourceTypeIP,
	AzureResourceTypeDISK,
	AzureResourceTypeVM,
}

var RequiredServices = []ServiceType{
	ServiceTypeSSH,
	ServiceTypeDocker,
	ServiceTypeBacalhau,
}

var SkippedResourceTypes = []string{
	"Microsoft.Compute/virtualMachines/extensions",
}

type AzureResourceTypes struct {
	ResourceString    string
	ShortResourceName string
}

func (a *AzureResourceTypes) GetResourceLowerString() string {
	return strings.ToLower(a.ResourceString)
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
	UniqueLocations       []string
	OrchestratorIP        string
	Machines              map[string]*Machine
	UniqueLocations       []string
	ProjectID             string
	UniqueID              string
	Tags                  map[string]*string
	AllowedPorts          []int
	SSHUser               string
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
	deploymentMutex       sync.RWMutex
}

type Disk struct {
	Name   string
	ID     string
	SizeGB int32
	State  armcompute.DiskState
}

func NewDeployment() *Deployment {
	return &Deployment{
		StartTime: time.Now(),
		Machines:  make(map[string]*Machine),
		Tags:      make(map[string]*string),
	}
}

func (d *Deployment) ToMap() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return map[string]interface{}{
		"ResourceGroupName":     d.ResourceGroupName,
		"ResourceGroupLocation": d.ResourceGroupLocation,
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
	viperMachines := make(map[string]map[string]interface{})
	for _, machine := range d.Machines {
		viperMachines[machine.Name] = map[string]interface{}{
			"Name":         machine.Name,
			"PublicIP":     machine.PublicIP,
			"PrivateIP":    machine.PrivateIP,
			"Orchestrator": machine.Parameters.Orchestrator,
		}
	}

	v.Set(deploymentPath, viperMachines)
	return v.WriteConfig()
}

func (d *Deployment) GetMachine(name string) *Machine {
	d.deploymentMutex.RLock()
	defer d.deploymentMutex.RUnlock()
	if machine, ok := d.Machines[name]; ok {
		return machine
	}
	return nil
}

func (d *Deployment) UpdateMachine(name string, updater func(*Machine)) error {
	d.deploymentMutex.Lock()
	defer d.deploymentMutex.Unlock()
	if machine, ok := d.Machines[name]; ok {
		updater(machine)
		return nil
	}
	return fmt.Errorf("machine %s not found", name)
}

type StatusUpdateMsg struct {
	Status *DisplayStatus
}
