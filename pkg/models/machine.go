package models

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	internal_azure "github.com/bacalhau-project/andaime/internal/clouds/azure"
	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

type Machine struct {
	ID       string
	Name     string
	Type     ResourceTypes
	Location string
	Region   string
	Zone     string

	StatusMessage string
	Parameters    Parameters
	PublicIP      string
	PrivateIP     string
	StartTime     time.Time

	VMSize         string
	DiskSizeGB     int32 `default:"30"`
	DiskImage      string
	ElapsedTime    time.Duration
	Orchestrator   bool
	OrchestratorIP string

	SSHUser               string
	SSHPrivateKeyMaterial []byte
	SSHPrivateKeyPath     string
	SSHPublicKeyMaterial  []byte
	SSHPublicKeyPath      string
	SSHPort               int

	CreationStartTime time.Time
	CreationEndTime   time.Time
	SSHStartTime      time.Time
	SSHEndTime        time.Time
	DockerStartTime   time.Time
	DockerEndTime     time.Time
	BacalhauStartTime time.Time
	BacalhauEndTime   time.Time
	DeploymentEndTime time.Time

	machineResources map[string]MachineResource
	machineServices  map[string]ServiceType

	stateMutex sync.RWMutex
	done       bool

	DeploymentType DeploymentType
	CloudProvider  DeploymentType
	CloudSpecific  CloudSpecificInfo
}

type CloudSpecificInfo struct {
	// Azure specific
	ResourceGroupName string

	// GCP specific
	Zone   string
	Region string
}

func NewMachine(
	cloudProvider DeploymentType,
	location string,
	vmSize string,
	diskSizeGB int32,
) (*Machine, error) {
	newID := utils.CreateShortID()

	if !IsValidLocation(cloudProvider, location) {
		return nil, fmt.Errorf("invalid location for %s: %s", cloudProvider, location)
	}

	machineName := fmt.Sprintf("%s-vm", newID)
	returnMachine := &Machine{
		ID:            machineName,
		Name:          machineName,
		StartTime:     time.Now(),
		Location:      location,
		VMSize:        vmSize,
		DiskSizeGB:    diskSizeGB,
		CloudProvider: cloudProvider,
		CloudSpecific: CloudSpecificInfo{},
	}

	for _, service := range RequiredServices {
		returnMachine.SetServiceState(service.Name, ServiceStateNotStarted)
	}

	if cloudProvider == DeploymentTypeAzure {
		for _, resource := range RequiredAzureResources {
			returnMachine.SetResource(resource.GetResourceLowerString(), ResourceStateNotStarted)
		}
	} else if cloudProvider == DeploymentTypeGCP {
		for _, resource := range RequiredGCPResources {
			returnMachine.SetResource(resource.GetResourceLowerString(), ResourceStateNotStarted)
		}
		returnMachine.Type = GCPResourceTypeInstance
	}

	return returnMachine, nil
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

func (m *Machine) SetLocation(location string) error {
	switch m.DeploymentType {
	case DeploymentTypeAzure:
		if !internal_azure.IsValidAzureLocation(location) {
			return fmt.Errorf("invalid Azure location: %s", location)
		}
	case DeploymentTypeGCP:
		if !internal_gcp.IsValidGCPLocation(location) {
			return fmt.Errorf("invalid GCP location: %s", location)
		}
	default:
		return fmt.Errorf("unknown deployment type: %s", m.DeploymentType)
	}
	m.Location = location
	return nil
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
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.done
}

func (m *Machine) SetComplete() {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	if !m.done {
		m.done = true
		m.DeploymentEndTime = time.Now()
	}
}

func (m *Machine) IsComplete() bool {
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

func (m *Machine) SetResource(resourceType string, resourceState ResourceState) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.setResourceUnsafe(resourceType, resourceState)
}

func (m *Machine) setResourceUnsafe(resourceType string, resourceState ResourceState) {
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

func (m *Machine) GetResourceState(resourceName string) ResourceState {
	return m.GetResource(resourceName).ResourceState
}

func (m *Machine) SetResourceState(resourceName string, state ResourceState) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	if m.machineResources == nil {
		m.machineResources = make(map[string]MachineResource)
	}

	resourceNameLower := strings.ToLower(resourceName)
	if resource, ok := m.machineResources[resourceNameLower]; ok {
		resource.ResourceState = state
		m.machineResources[resourceNameLower] = resource
	} else {
		m.machineResources[resourceNameLower] = MachineResource{
			ResourceName:  resourceName,
			ResourceState: state,
		}
	}
}

func (m *Machine) ResourcesComplete() (int, int) {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.countCompletedResources()
}

func (m *Machine) countCompletedResources() (int, int) {
	completedResources := 0

	requiredResources := []ResourceTypes{}
	if m.CloudProvider == DeploymentTypeAzure {
		requiredResources = RequiredAzureResources
	} else if m.CloudProvider == DeploymentTypeGCP {
		requiredResources = RequiredGCPResources
	}

	for _, requiredResource := range requiredResources {
		resource := m.getResourceUnsafe(requiredResource.GetResourceLowerString())
		if resource.ResourceState == ResourceStateSucceeded ||
			resource.ResourceState == ResourceStateRunning {
			completedResources++
		}
	}

	return completedResources, len(requiredResources)
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

func (m *Machine) InstallDockerAndCorePackages(ctx context.Context) error {
	if err := m.installDocker(ctx); err != nil {
		return err
	}

	if err := m.installCorePackages(ctx); err != nil {
		return err
	}

	return nil
}

func (m *Machine) installDocker(ctx context.Context) error {
	return m.installService(
		ctx,
		"Docker",
		"install-docker.sh",
		general.GetInstallDockerScript,
	)
}

func (m *Machine) installCorePackages(ctx context.Context) error {
	return m.installService(
		ctx,
		"CorePackages",
		"install-core-packages.sh",
		general.GetInstallCorePackagesScript,
	)
}

func (m *Machine) installService(
	ctx context.Context,
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
	if err := sshConfig.PushFile(ctx, scriptPath, scriptBytes, true); err != nil {
		return err
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
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

// Add this new function
func IsValidLocation(deploymentType DeploymentType, location string) bool {
	switch deploymentType {
	case DeploymentTypeAzure:
		return internal_azure.IsValidAzureLocation(location)
	case DeploymentTypeGCP:
		return internal_gcp.IsValidGCPLocation(location)
	default:
		return false
	}
}

// CloudType returns the cloud provider type for this machine
func (m *Machine) CloudType() DeploymentType {
	return m.CloudProvider
}
