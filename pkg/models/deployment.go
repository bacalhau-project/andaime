package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	internal "github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
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

	MachineResources map[string]MachineResource
	MachineServices  map[string]ServiceType

	VMSize         string
	DiskSizeGB     int32 `default:"30"`
	ComputerName   string
	ElapsedTime    time.Duration
	Orchestrator   bool
	OrchestratorIP string

	// New SSH-related fields
	SSHUser               string
	SSHPrivateKeyMaterial []byte
	SSHPort               int

	// Timing information
	CreationStartTime time.Time
	CreationEndTime   time.Time
	SSHStartTime      time.Time
	SSHEndTime        time.Time
	DockerStartTime   time.Time
	DockerEndTime     time.Time
	BacalhauStartTime time.Time
	BacalhauEndTime   time.Time
}

func (m *Machine) LogTimingInfo(logger *logger.Logger) {
	logger.Info(fmt.Sprintf("Machine %s timing information:", m.Name))
	logger.Info(fmt.Sprintf("  Creation time: %v", m.CreationEndTime.Sub(m.CreationStartTime)))
	logger.Info(fmt.Sprintf("  SSH setup time: %v", m.SSHEndTime.Sub(m.SSHStartTime)))
	logger.Info(
		fmt.Sprintf("  Docker installation time: %v", m.DockerEndTime.Sub(m.DockerStartTime)),
	)
	logger.Info(
		fmt.Sprintf("  Bacalhau setup time: %v", m.BacalhauEndTime.Sub(m.BacalhauStartTime)),
	)
	logger.Info(fmt.Sprintf("  Total time: %v", m.BacalhauEndTime.Sub(m.CreationStartTime)))
}

func (m *Machine) IsOrchestrator() bool {
	return m.Orchestrator
}

func (m *Machine) SSHEnabled() bool {
	return m.MachineServices["SSH"].State == ServiceStateSucceeded
}

func (m *Machine) DockerEnabled() bool {
	return m.MachineServices["Docker"].State == ServiceStateSucceeded
}

func (m *Machine) BacalhauEnabled() bool {
	return m.MachineServices["Bacalhau"].State == ServiceStateSucceeded
}

func (m *Machine) GetResource(resourceType string) MachineResource {
	if m.MachineResources == nil {
		m.MachineResources = make(map[string]MachineResource)
	}

	resourceTypeLower := strings.ToLower(resourceType)
	if resource, ok := m.MachineResources[resourceTypeLower]; ok {
		return resource
	} else {
		return MachineResource{}
	}
}

func (m *Machine) SetResource(resourceType string, resourceState AzureResourceState) {
	if m.MachineResources == nil {
		m.MachineResources = make(map[string]MachineResource)
	}
	resourceTypeLower := strings.ToLower(resourceType)
	m.MachineResources[resourceTypeLower] = MachineResource{
		ResourceName:  resourceType,
		ResourceType:  GetAzureResourceType(resourceType),
		ResourceState: resourceState,
		ResourceValue: "",
	}
}

func (m *Machine) ResourcesComplete() (int, int) {
	// l := logger.Get()
	totalResources := len(RequiredAzureResources)
	completedResources := 0

	for _, requiredResource := range RequiredAzureResources {
		if resource, exists := m.MachineResources[requiredResource.GetResourceLowerString()]; exists {
			if resource.ResourceState == AzureResourceStateSucceeded {
				completedResources++
			}
		}
	}

	// // Print completed and pending resources
	// l.Debugf("Completed resources:")
	// for _, requiredResource := range RequiredAzureResources {
	// 	if resource, exists := m.MachineResources[requiredResource.GetResourceLowerString()]; exists {
	// 		if resource.ResourceState == AzureResourceStateSucceeded {
	// 			l.Debugf("  %s", requiredResource.ResourceString)
	// 		}
	// 	}
	// }
	// l.Infof("Pending resources:")
	// for _, requiredResource := range RequiredAzureResources {
	// 	if resource, exists := m.MachineResources[requiredResource.GetResourceLowerString()]; exists {
	// 		if resource.ResourceState != AzureResourceStateSucceeded {
	// 			l.Infof("  %s", requiredResource.ResourceString)
	// 		}
	// 	} else {
	// 		l.Infof("  %s", requiredResource.ResourceString)
	// 	}
	// }

	return completedResources, totalResources
}

func (m *Machine) ServicesComplete() (int, int) {
	totalServices := len(RequiredServices)
	completedServices := 0

	for _, requiredService := range RequiredServices {
		if service, exists := m.MachineServices[requiredService.Name]; exists {
			if service.State == ServiceStateSucceeded {
				completedServices++
			}
		}
	}

	return completedServices, totalServices
}

func (m *Machine) Complete() bool {
	resourcesPending, resourcesTotal := m.ResourcesComplete()
	resourcesComplete := (resourcesPending == resourcesTotal) && (resourcesPending > 0)

	servicesPending, servicesTotal := m.ServicesComplete()
	servicesComplete := (servicesPending == servicesTotal) && (servicesPending > 0)

	return resourcesComplete &&
		servicesComplete
}

func (m *Machine) InstallDockerAndCorePackages() error {
	l := logger.Get()
	// Install Docker
	dockerService := m.MachineServices["Docker"]
	dockerService.State = ServiceStateUpdating
	m.MachineServices["Docker"] = dockerService

	sshConfig, err := sshutils.NewSSHConfig(
		m.PublicIP,
		m.SSHPort,
		m.SSHUser,
		m.SSHPrivateKeyMaterial,
	)
	if err != nil {
		l.Errorf("Error creating SSH config: %v", err)
		return err
	}

	installDockerScriptRemotePath := "/tmp/install-docker.sh"
	installDockerScriptBytes, err := internal.GetInstallDockerScript()
	if err != nil {
		return err
	}
	err = sshConfig.PushFile(installDockerScriptBytes, installDockerScriptRemotePath, true)
	if err != nil {
		return err
	}

	_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", installDockerScriptRemotePath))
	if err != nil {
		dockerService.State = ServiceStateFailed
		m.MachineServices["Docker"] = dockerService
		return err
	}

	dockerService.State = ServiceStateSucceeded
	m.MachineServices["Docker"] = dockerService

	corePackagesService := m.MachineServices["CorePackages"]
	corePackagesService.State = ServiceStateUpdating
	m.MachineServices["CorePackages"] = corePackagesService

	installCorePackagesScriptPath := "/tmp/install-core-packages.sh"
	corePackagesScriptBytes, err := internal.GetInstallCorePackagesScript()
	if err != nil {
		return err
	}
	err = sshConfig.PushFile(corePackagesScriptBytes, installCorePackagesScriptPath, true)
	if err != nil {
		return err
	}

	_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", installCorePackagesScriptPath))
	if err != nil {
		corePackagesService.State = ServiceStateFailed
		m.MachineServices["CorePackages"] = corePackagesService
		return err
	}

	corePackagesService.State = ServiceStateSucceeded
	m.MachineServices["CorePackages"] = corePackagesService
	return nil
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
	OrchestratorNode      *Machine
	OrchestratorIP        string
	Machines              []Machine
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
