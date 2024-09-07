package models

import (
	"context"
	"fmt"
	"math/rand"
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

	VMSize          string
	DiskSizeGB      int32 `default:"30"`
	DiskImageFamily string
	DiskImageURL    string
	ElapsedTime     time.Duration
	Orchestrator    bool
	OrchestratorIP  string

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

func (mach *Machine) LogTimingInfo(logger *logger.Logger) {
	logger.Info(fmt.Sprintf("Machine %s timing information:", mach.Name))
	logTimingDetail(logger, "Creation time", mach.CreationStartTime, mach.CreationEndTime)
	logTimingDetail(logger, "SSH setup time", mach.SSHStartTime, mach.SSHEndTime)
	logTimingDetail(logger, "Docker installation time", mach.DockerStartTime, mach.DockerEndTime)
	logTimingDetail(logger, "Bacalhau setup time", mach.BacalhauStartTime, mach.BacalhauEndTime)
	logger.Info(fmt.Sprintf("  Total time: %v", mach.BacalhauEndTime.Sub(mach.CreationStartTime)))
}

func logTimingDetail(logger *logger.Logger, label string, start, end time.Time) {
	logger.Info(fmt.Sprintf("  %s: %v", label, end.Sub(start)))
}

// Status check methods

func (mach *Machine) IsOrchestrator() bool {
	return mach.Orchestrator
}

func (mach *Machine) SetLocation(location string) error {
	switch mach.DeploymentType {
	case DeploymentTypeAzure:
		if !internal_azure.IsValidAzureLocation(location) {
			return fmt.Errorf("invalid Azure location: %s", location)
		}
	case DeploymentTypeGCP:
		if !internal_gcp.IsValidGCPLocation(location) {
			return fmt.Errorf("invalid GCP location: %s", location)
		}
	default:
		return fmt.Errorf("unknown deployment type: %s", mach.DeploymentType)
	}
	mach.Location = location
	return nil
}

func (mach *Machine) SSHEnabled() bool {
	return mach.GetServiceState("SSH") == ServiceStateSucceeded
}

func (mach *Machine) DockerEnabled() bool {
	return mach.GetServiceState("Docker") == ServiceStateSucceeded
}

func (mach *Machine) BacalhauEnabled() bool {
	return mach.GetServiceState("Bacalhau") == ServiceStateSucceeded
}

func (mach *Machine) Complete() bool {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.done
}

func (mach *Machine) SetComplete() {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	if !mach.done {
		mach.done = true
		mach.DeploymentEndTime = time.Now()
	}
}

func (mach *Machine) IsComplete() bool {
	resourcesComplete := mach.resourcesComplete()
	servicesComplete := mach.servicesComplete()
	return resourcesComplete && servicesComplete
}

func (mach *Machine) resourcesComplete() bool {
	completed, total := mach.ResourcesComplete()
	return (completed == total) && (total > 0)
}

func (mach *Machine) servicesComplete() bool {
	completed, total := mach.ServicesComplete()
	return (completed == total) && (total > 0)
}

// Resource management methods

func (mach *Machine) GetResource(resourceType string) MachineResource {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.getResourceUnsafe(resourceType)
}

func (mach *Machine) getResourceUnsafe(resourceType string) MachineResource {
	if mach.machineResources == nil {
		mach.machineResources = make(map[string]MachineResource)
	}

	resourceTypeLower := strings.ToLower(resourceType)
	if resource, ok := mach.machineResources[resourceTypeLower]; ok {
		return resource
	}
	return MachineResource{}
}

func (mach *Machine) SetResource(resourceType string, resourceState ResourceState) {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	mach.setResourceUnsafe(resourceType, resourceState)
}

func (mach *Machine) setResourceUnsafe(resourceType string, resourceState ResourceState) {
	if mach.machineResources == nil {
		mach.machineResources = make(map[string]MachineResource)
	}
	resourceTypeLower := strings.ToLower(resourceType)
	mach.machineResources[resourceTypeLower] = MachineResource{
		ResourceName:  resourceType,
		ResourceType:  GetAzureResourceType(resourceType),
		ResourceState: resourceState,
		ResourceValue: "",
	}
}

func (mach *Machine) GetResourceState(resourceName string) ResourceState {
	return mach.GetResource(resourceName).ResourceState
}

func (mach *Machine) SetResourceState(resourceName string, state ResourceState) {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	if mach.machineResources == nil {
		mach.machineResources = make(map[string]MachineResource)
	}

	resourceNameLower := strings.ToLower(resourceName)
	if resource, ok := mach.machineResources[resourceNameLower]; ok {
		resource.ResourceState = state
		mach.machineResources[resourceNameLower] = resource
	} else {
		mach.machineResources[resourceNameLower] = MachineResource{
			ResourceName:  resourceName,
			ResourceState: state,
		}
	}
}

func (mach *Machine) ResourcesComplete() (int, int) {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.countCompletedResources()
}

func (mach *Machine) countCompletedResources() (int, int) {
	l := logger.Get()
	completedResources := 0

	requiredResources := []ResourceTypes{}
	if mach.CloudProvider == DeploymentTypeAzure {
		requiredResources = RequiredAzureResources
	} else if mach.CloudProvider == DeploymentTypeGCP {
		requiredResources = RequiredGCPResources
	}

	for _, requiredResource := range requiredResources {
		resource := mach.getResourceUnsafe(requiredResource.GetResourceLowerString())
		if resource.ResourceState == ResourceStateSucceeded ||
			resource.ResourceState == ResourceStateRunning {
			// l.Debugf("Machine %s: Resource %s is completed", m.Name, resource.ResourceName)
			completedResources++
		} else {
			// Print out roughly every 1000 calls
			//nolint:gosec,mnd
			if rand.Intn(1000) < 10 {
				l.Debugf("Machine %s: Resource %s is not completed", mach.Name, resource.ResourceName)
			}
		}
	}

	return completedResources, len(requiredResources)
}

// Service management methods

func (mach *Machine) GetServiceState(serviceName string) ServiceState {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.getServiceStateUnsafe(serviceName)
}

func (mach *Machine) getServiceStateUnsafe(serviceName string) ServiceState {
	if mach.machineServices == nil {
		mach.machineServices = make(map[string]ServiceType)
		return ServiceStateUnknown
	}

	if service, ok := mach.machineServices[serviceName]; ok {
		return service.State
	}
	return ServiceStateUnknown
}

func (mach *Machine) SetServiceState(serviceName string, state ServiceState) {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	mach.setServiceStateUnsafe(serviceName, state)
}

func (mach *Machine) setServiceStateUnsafe(serviceName string, state ServiceState) {
	if mach.machineServices == nil {
		mach.machineServices = make(map[string]ServiceType)
	}

	if service, ok := mach.machineServices[serviceName]; ok {
		service.State = state
		mach.machineServices[serviceName] = service
	} else {
		mach.machineServices[serviceName] = ServiceType{Name: serviceName, State: state}
	}
}

func (mach *Machine) ServicesComplete() (int, int) {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.countCompletedServices(RequiredServices)
}

func (mach *Machine) countCompletedServices(requiredServices []ServiceType) (int, int) {
	completedServices := 0
	totalServices := len(requiredServices)

	for _, requiredService := range requiredServices {
		if mach.getServiceStateUnsafe(requiredService.Name) == ServiceStateSucceeded {
			completedServices++
		}
	}

	return completedServices, totalServices
}

// Machine Services
func (mach *Machine) GetMachineServices() map[string]ServiceType {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.machineServices
}

func (mach *Machine) GetMachineResources() map[string]MachineResource {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.machineResources
}

func (mach *Machine) SetMachineServices(services map[string]ServiceType) {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	mach.machineServices = services
}

// Installation methods

func (mach *Machine) InstallDockerAndCorePackages(ctx context.Context) error {
	l := logger.Get()
	l.Debugf("Installing Docker and core packages on machine %s", mach.Name)

	mach.SetServiceState("Docker", ServiceStateUpdating)
	if err := mach.installDocker(ctx); err != nil {
		mach.SetServiceState("Docker", ServiceStateFailed)
		return err
	}

	if err := mach.verifyDocker(ctx); err != nil {
		mach.SetServiceState("Docker", ServiceStateFailed)
		return err
	}
	mach.SetServiceState("Docker", ServiceStateSucceeded)

	mach.SetServiceState("CorePackages", ServiceStateUpdating)

	if err := mach.installCorePackages(ctx); err != nil {
		mach.SetServiceState("CorePackages", ServiceStateFailed)
		return err
	}
	mach.SetServiceState("CorePackages", ServiceStateSucceeded)

	return nil
}

func (mach *Machine) installDocker(ctx context.Context) error {
	return mach.installService(
		ctx,
		"Docker",
		"install-docker.sh",
		general.GetInstallDockerScript,
	)
}

func (mach *Machine) verifyDocker(ctx context.Context) error {
	l := logger.Get()

	sshConfig, err := sshutils.NewSSHConfigFunc(
		mach.PublicIP,
		mach.SSHPort,
		mach.SSHUser,
		mach.SSHPrivateKeyMaterial,
	)
	if err != nil {
		l.Errorf("Error creating SSH config: %v", err)
		return err
	}

	output, err := sshConfig.ExecuteCommand(ctx, "sudo docker run hello-world")
	if err != nil {
		l.Errorf("Failed to verify Docker on machine %s: %v", mach.Name, err)
		return err
	}

	if strings.Contains(output, "Hello from Docker!") {
		l.Errorf("Failed to verify Docker on machine %s: %v", mach.Name, err)
		return err
	}

	return nil
}

func (mach *Machine) installCorePackages(ctx context.Context) error {
	return mach.installService(
		ctx,
		"CorePackages",
		"install-core-packages.sh",
		general.GetInstallCorePackagesScript,
	)
}

func (mach *Machine) installService(
	ctx context.Context,
	serviceName, scriptName string,
	scriptGetter func() ([]byte, error),
) error {
	l := logger.Get()
	mach.SetServiceState(serviceName, ServiceStateUpdating)

	sshConfig, err := sshutils.NewSSHConfigFunc(
		mach.PublicIP,
		mach.SSHPort,
		mach.SSHUser,
		mach.SSHPrivateKeyMaterial,
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
		mach.SetServiceState(serviceName, ServiceStateFailed)
		return err
	}

	mach.SetServiceState(serviceName, ServiceStateSucceeded)
	return nil
}

func (mach *Machine) EnsureMachineServices() error {
	if mach.machineServices == nil {
		mach.machineServices = make(map[string]ServiceType)
	}
	for _, service := range RequiredServices {
		mach.SetServiceState(service.Name, ServiceStateNotStarted)
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
func (mach *Machine) CloudType() DeploymentType {
	return mach.CloudProvider
}
