package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	internal "github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
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

	VMSize         string
	DiskSizeGB     int32 `default:"30"`
	ElapsedTime    time.Duration
	Orchestrator   bool
	OrchestratorIP string

	SSHUser               string
	SSHPrivateKeyMaterial []byte
	SSHPort               int

	CreationStartTime time.Time
	CreationEndTime   time.Time
	SSHStartTime      time.Time
	SSHEndTime        time.Time
	DockerStartTime   time.Time
	DockerEndTime     time.Time
	BacalhauStartTime time.Time
	BacalhauEndTime   time.Time

	machineResources map[string]MachineResource
	machineServices  map[string]ServiceType

	stateMutex sync.RWMutex
}

// Logging methods

func (m *Machine) LogTimingInfo(logger *logger.Logger) {
	logger.Info(fmt.Sprintf("Machine %s timing information:", m.Name))
	logTimingDetail(logger, "Creation time", m.CreationStartTime, m.CreationEndTime)
	logTimingDetail(logger, "SSH setup time", m.SSHStartTime, m.SSHEndTime)
	logTimingDetail(logger, "Docker installation time", m.DockerStartTime, m.DockerEndTime)
	logTimingDetail(logger, "Bacalhau setup time", m.BacalhauStartTime, m.BacalhauEndTime)
	logger.Info(fmt.Sprintf("  Total time: %v", m.BacalhauEndTime.Sub(m.CreationStartTime)))
}

func logTimingDetail(logger *logger.Logger, label string, start, end time.Time) {
	logger.Info(fmt.Sprintf("  %s: %v", label, end.Sub(start)))
}

// Status check methods

func (m *Machine) IsOrchestrator() bool {
	return m.Orchestrator
}

func (m *Machine) SSHEnabled() bool {
	return m.GetServiceState("SSH") == ServiceStateSucceeded
}

func (m *Machine) DockerEnabled() bool {
	return m.GetServiceState("Docker") == ServiceStateSucceeded
}

func (m *Machine) BacalhauEnabled() bool {
	return m.GetServiceState("Bacalhau") == ServiceStateSucceeded
}

func (m *Machine) Complete() bool {
	resourcesComplete := m.resourcesComplete()
	servicesComplete := m.servicesComplete()
	return resourcesComplete && servicesComplete
}

func (m *Machine) resourcesComplete() bool {
	completed, total := m.ResourcesComplete()
	return (completed == total) && (total > 0)
}

func (m *Machine) servicesComplete() bool {
	completed, total := m.ServicesComplete()
	return (completed == total) && (total > 0)
}

// Resource management methods

func (m *Machine) GetResource(resourceType string) MachineResource {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.getResourceUnsafe(resourceType)
}

func (m *Machine) getResourceUnsafe(resourceType string) MachineResource {
	if m.machineResources == nil {
		m.machineResources = make(map[string]MachineResource)
	}

	resourceTypeLower := strings.ToLower(resourceType)
	if resource, ok := m.machineResources[resourceTypeLower]; ok {
		return resource
	}
	return MachineResource{}
}

func (m *Machine) SetResource(resourceType string, resourceState AzureResourceState) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.setResourceUnsafe(resourceType, resourceState)
}

func (m *Machine) setResourceUnsafe(resourceType string, resourceState AzureResourceState) {
	if m.machineResources == nil {
		m.machineResources = make(map[string]MachineResource)
	}
	resourceTypeLower := strings.ToLower(resourceType)
	m.machineResources[resourceTypeLower] = MachineResource{
		ResourceName:  resourceType,
		ResourceType:  GetAzureResourceType(resourceType),
		ResourceState: resourceState,
		ResourceValue: "",
	}
}

func (m *Machine) GetResourceState(resourceName string) AzureResourceState {
	return m.GetResource(resourceName).ResourceState
}

func (m *Machine) SetResourceState(resourceName string, state AzureResourceState) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	if resource, ok := m.machineResources[resourceName]; ok {
		resource.ResourceState = state
		m.machineResources[resourceName] = resource
	}
}

func (m *Machine) ResourcesComplete() (int, int) {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.countCompletedResources(RequiredAzureResources)
}

func (m *Machine) countCompletedResources(requiredResources []AzureResourceTypes) (int, int) {
	completedResources := 0
	totalResources := len(requiredResources)

	for _, requiredResource := range requiredResources {
		if resource := m.getResourceUnsafe(requiredResource.GetResourceLowerString()); resource.ResourceState == AzureResourceState(
			ServiceStateSucceeded,
		) {
			completedResources++
		}
	}

	return completedResources, totalResources
}

// Service management methods

func (m *Machine) GetServiceState(serviceName string) ServiceState {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.getServiceStateUnsafe(serviceName)
}

func (m *Machine) getServiceStateUnsafe(serviceName string) ServiceState {
	if m.machineServices == nil {
		m.machineServices = make(map[string]ServiceType)
		return ServiceStateUnknown
	}

	if service, ok := m.machineServices[serviceName]; ok {
		return service.State
	}
	return ServiceStateUnknown
}

func (m *Machine) SetServiceState(serviceName string, state ServiceState) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.setServiceStateUnsafe(serviceName, state)
}

func (m *Machine) setServiceStateUnsafe(serviceName string, state ServiceState) {
	if m.machineServices == nil {
		m.machineServices = make(map[string]ServiceType)
	}

	if service, ok := m.machineServices[serviceName]; ok {
		service.State = state
		m.machineServices[serviceName] = service
	} else {
		m.machineServices[serviceName] = ServiceType{Name: serviceName, State: state}
	}
}

func (m *Machine) ServicesComplete() (int, int) {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.countCompletedServices(RequiredServices)
}

func (m *Machine) countCompletedServices(requiredServices []ServiceType) (int, int) {
	completedServices := 0
	totalServices := len(requiredServices)

	for _, requiredService := range requiredServices {
		if m.getServiceStateUnsafe(requiredService.Name) == ServiceStateSucceeded {
			completedServices++
		}
	}

	return completedServices, totalServices
}

// Machine Services
func (m *Machine) GetMachineServices() map[string]ServiceType {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.machineServices
}

func (m *Machine) GetMachineResources() map[string]MachineResource {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.machineResources
}

func (m *Machine) SetMachineServices(services map[string]ServiceType) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.machineServices = services
}

// Installation methods

func (m *Machine) InstallDockerAndCorePackages() error {
	if err := m.installDocker(); err != nil {
		return err
	}
	return m.installCorePackages()
}

func (m *Machine) installDocker() error {
	return m.installService("Docker", "install-docker.sh", internal.GetInstallDockerScript)
}

func (m *Machine) installCorePackages() error {
	return m.installService(
		"CorePackages",
		"install-core-packages.sh",
		internal.GetInstallCorePackagesScript,
	)
}

func (m *Machine) installService(
	serviceName, scriptName string,
	scriptGetter func() ([]byte, error),
) error {
	l := logger.Get()
	m.SetServiceState(serviceName, ServiceStateUpdating)

	sshConfig, err := sshutils.NewSSHConfigFunc(
		m.PublicIP,
		m.SSHPort,
		m.SSHUser,
		m.SSHPrivateKeyMaterial,
	)
	if err != nil {
		l.Errorf("Error creating SSH config: %v", err)
		return err
	}

	scriptBytes, err := scriptGetter()
	if err != nil {
		return err
	}

	scriptPath := fmt.Sprintf("/tmp/%s", scriptName)
	if err := sshConfig.PushFile(scriptBytes, scriptPath, true); err != nil {
		return err
	}

	if _, err := sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		m.SetServiceState(serviceName, ServiceStateFailed)
		return err
	}

	m.SetServiceState(serviceName, ServiceStateSucceeded)
	return nil
}

func (m *Machine) EnsureMachineServices() error {
	if m.machineServices == nil {
		m.machineServices = make(map[string]ServiceType)
	}
	for _, service := range RequiredServices {
		m.SetServiceState(service.Name, ServiceStateNotStarted)
	}
	return nil
}
