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

type Machiner interface {
	// Basic information
	GetID() string
	GetName() string
	GetType() ResourceType
	GetVMSize() string
	GetDiskSizeGB() int
	SetDiskSizeGB(size int)
	GetDiskImageFamily() string
	SetDiskImageFamily(family string)
	GetDiskImageURL() string
	SetDiskImageURL(url string)
	GetLocation() string
	SetLocation(location string) error
	GetRegion() string
	GetZone() string
	GetPublicIP() string
	SetPublicIP(ip string)
	GetPrivateIP() string
	SetPrivateIP(ip string)

	// SSH related
	GetSSHUser() string
	SetSSHUser(user string)
	GetSSHPublicKeyMaterial() []byte
	SetSSHPublicKeyMaterial([]byte)
	GetSSHPublicKeyPath() string
	GetSSHPrivateKeyMaterial() []byte
	SetSSHPrivateKeyMaterial([]byte)
	GetSSHPrivateKeyPath() string
	SetSSHPrivateKeyPath(path string)
	GetSSHPort() int
	SetSSHPort(port int)

	// Status and state
	IsOrchestrator() bool
	SetOrchestrator(orchestrator bool)
	IsComplete() bool
	SetComplete()
	GetOrchestratorIP() string
	SetOrchestratorIP(ip string)
	GetStatusMessage() string
	SetStatusMessage(message string)

	// Resource management
	GetMachineResource(resourceType string) MachineResource
	SetMachineResource(resourceType string, resourceState MachineResourceState)
	GetMachineResources() map[string]MachineResource
	GetMachineResourceState(resourceName string) MachineResourceState
	SetMachineResourceState(resourceName string, state MachineResourceState)
	ResourcesComplete() (int, int)

	// Service management
	GetServices() map[string]ServiceType
	GetServiceState(serviceName string) ServiceState
	SetServiceState(serviceName string, state ServiceState)
	EnsureMachineServices() error
	ServicesComplete() (int, int)

	// Installation methods
	InstallDockerAndCorePackages(ctx context.Context) error

	// Cloud provider
	CloudType() DeploymentType

	// Timing information
	GetStartTime() time.Time
	SetStartTime(startTime time.Time)
	GetDeploymentEndTime() time.Time
	SetDeploymentEndTime(endTime time.Time)
	GetElapsedTime() time.Duration
	SetElapsedTime(elapsedTime time.Duration)

	// Logging
	LogTimingInfo(logger logger.Logger)
}

type Machine struct {
	ID       string
	Name     string
	Type     ResourceType
	Location string
	Region   string
	Zone     string

	StatusMessage string
	Parameters    Parameters
	PublicIP      string
	PrivateIP     string
	StartTime     time.Time

	VMSize          string
	DiskSizeGB      int
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
	diskSizeGB int,
	cloudSpecificInfo CloudSpecificInfo,
) (Machiner, error) {
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
		CloudSpecific: cloudSpecificInfo,
	}

	for _, service := range RequiredServices {
		returnMachine.SetServiceState(service.Name, ServiceStateNotStarted)
	}

	if cloudProvider == DeploymentTypeAzure {
		for _, resource := range RequiredAzureResources {
			returnMachine.SetMachineResource(
				resource.GetResourceLowerString(),
				ResourceStateNotStarted,
			)
		}
	} else if cloudProvider == DeploymentTypeGCP {
		for _, resource := range RequiredGCPResources {
			returnMachine.SetMachineResource(
				resource.GetResourceLowerString(),
				ResourceStateNotStarted,
			)
		}
		returnMachine.Type = GCPResourceTypeInstance
	}

	return returnMachine, nil
}

// Logging methods

func (mach *Machine) LogTimingInfo(logger logger.Logger) {
	logger.Info(fmt.Sprintf("Machine %s timing information:", mach.Name))
	logTimingDetail(logger, "Creation time", mach.CreationStartTime, mach.CreationEndTime)
	logTimingDetail(logger, "SSH setup time", mach.SSHStartTime, mach.SSHEndTime)
	logTimingDetail(logger, "Docker installation time", mach.DockerStartTime, mach.DockerEndTime)
	logTimingDetail(logger, "Bacalhau setup time", mach.BacalhauStartTime, mach.BacalhauEndTime)
	logger.Info(fmt.Sprintf("  Total time: %v", mach.BacalhauEndTime.Sub(mach.CreationStartTime)))
}

func logTimingDetail(logger logger.Logger, label string, start, end time.Time) {
	logger.Info(fmt.Sprintf("  %s: %v", label, end.Sub(start)))
}

// Status check methods

func (mach *Machine) IsOrchestrator() bool {
	return mach.Orchestrator
}

func (mach *Machine) SetOrchestrator(orchestrator bool) {
	mach.Orchestrator = orchestrator
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

func (mach *Machine) GetMachineResource(resourceType string) MachineResource {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.getMachineResourceUnsafe(resourceType)
}

func (mach *Machine) getMachineResourceUnsafe(resourceType string) MachineResource {
	if mach.machineResources == nil {
		mach.machineResources = make(map[string]MachineResource)
	}

	resourceTypeLower := strings.ToLower(resourceType)
	if resource, ok := mach.machineResources[resourceTypeLower]; ok {
		return resource
	}
	return MachineResource{}
}

func (mach *Machine) SetMachineResource(resourceType string, resourceState MachineResourceState) {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	mach.setMachineResourceUnsafe(resourceType, resourceState)
}

func (mach *Machine) setMachineResourceUnsafe(
	resourceType string,
	resourceState MachineResourceState,
) {
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

func (mach *Machine) GetMachineResourceState(resourceName string) MachineResourceState {
	return mach.GetMachineResource(resourceName).ResourceState
}

func (mach *Machine) SetMachineResourceState(resourceName string, state MachineResourceState) {
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

	requiredResources := []ResourceType{}
	if mach.CloudProvider == DeploymentTypeAzure {
		requiredResources = RequiredAzureResources
	} else if mach.CloudProvider == DeploymentTypeGCP {
		requiredResources = RequiredGCPResources
	}

	for _, requiredResource := range requiredResources {
		resource := mach.getMachineResourceUnsafe(requiredResource.GetResourceLowerString())
		if resource.ResourceState == ResourceStateSucceeded ||
			resource.ResourceState == ResourceStateRunning {
			// l.Debugf("Machine %s: Resource %s is completed", m.Name, resource.ResourceName)
			completedResources++
		} else {
			// The below is a no-op, but it's a good way to get some details
			// when debugging.
			// Print out roughly every 1000 calls
			//nolint:gosec,mnd
			if rand.Intn(1000) < -1 {
				l.Debugf("Machine %s: Resource %s is not completed", mach.Name, resource.ResourceName)
			}
		}
	}

	return completedResources, len(requiredResources)
}

// Service management methods
func (mach *Machine) GetServices() map[string]ServiceType {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.machineServices
}

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
		mach.SSHPrivateKeyPath,
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
		mach.SSHPrivateKeyPath,
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

// Add these methods to the Machine struct

func (mach *Machine) GetID() string {
	return mach.ID
}

func (mach *Machine) GetName() string {
	return mach.Name
}

func (mach *Machine) GetType() ResourceType {
	return mach.Type
}

func (mach *Machine) GetRegion() string {
	return mach.CloudSpecific.Region
}

func (mach *Machine) GetZone() string {
	return mach.CloudSpecific.Zone
}

func (mach *Machine) GetLocation() string {
	return mach.Location
}

func (mach *Machine) GetPublicIP() string {
	return mach.PublicIP
}

func (mach *Machine) SetPublicIP(ip string) {
	mach.PublicIP = ip
}

func (mach *Machine) GetPrivateIP() string {
	return mach.PrivateIP
}

func (mach *Machine) SetPrivateIP(ip string) {
	mach.PrivateIP = ip
}

func (mach *Machine) GetSSHUser() string {
	return mach.SSHUser
}

func (mach *Machine) SetSSHUser(user string) {
	mach.SSHUser = user
}

func (mach *Machine) GetSSHPrivateKeyMaterial() []byte {
	return mach.SSHPrivateKeyMaterial
}

func (mach *Machine) SetSSHPrivateKeyMaterial(key []byte) {
	mach.SSHPrivateKeyMaterial = key
}

func (mach *Machine) GetSSHPrivateKeyPath() string {
	return mach.SSHPrivateKeyPath
}

func (mach *Machine) SetSSHPrivateKeyPath(path string) {
	mach.SSHPrivateKeyPath = path
}

func (mach *Machine) GetSSHPublicKeyMaterial() []byte {
	return mach.SSHPublicKeyMaterial
}

func (mach *Machine) SetSSHPublicKeyMaterial(key []byte) {
	mach.SSHPublicKeyMaterial = key
}

func (mach *Machine) GetSSHPublicKeyPath() string {
	return mach.SSHPublicKeyPath
}

func (mach *Machine) SetSSHPublicKeyPath(path string) {
	mach.SSHPublicKeyPath = path
}

func (mach *Machine) GetSSHPort() int {
	return mach.SSHPort
}

func (mach *Machine) SetSSHPort(port int) {
	mach.SSHPort = port
}

func (mach *Machine) GetOrchestratorIP() string {
	return mach.OrchestratorIP
}

func (mach *Machine) SetOrchestratorIP(ip string) {
	mach.OrchestratorIP = ip
}

func (mach *Machine) GetStatusMessage() string {
	return mach.StatusMessage
}

func (mach *Machine) SetStatusMessage(message string) {
	mach.StatusMessage = message
}

func (mach *Machine) GetStartTime() time.Time {
	return mach.StartTime
}

func (mach *Machine) GetDeploymentEndTime() time.Time {
	return mach.DeploymentEndTime
}

func (mach *Machine) GetDiskImageFamily() string {
	return mach.DiskImageFamily
}

func (mach *Machine) SetDiskImageFamily(family string) {
	mach.DiskImageFamily = family
}

func (mach *Machine) GetDiskSizeGB() int {
	return mach.DiskSizeGB
}

func (mach *Machine) SetDiskSizeGB(size int) {
	mach.DiskSizeGB = size
}

func (mach *Machine) GetDiskImageURL() string {
	return mach.DiskImageURL
}

func (mach *Machine) SetDiskImageURL(url string) {
	mach.DiskImageURL = url
}

func (mach *Machine) GetElapsedTime() time.Duration {
	return mach.ElapsedTime
}

func (mach *Machine) SetElapsedTime(elapsedTime time.Duration) {
	mach.ElapsedTime = elapsedTime
}

func (mach *Machine) GetVMSize() string {
	return mach.VMSize
}

func (mach *Machine) GetVMCount() int {
	return 1
}

func (mach *Machine) SetStartTime(startTime time.Time) {
	mach.StartTime = startTime
}

func (mach *Machine) SetDeploymentEndTime(endTime time.Time) {
	mach.DeploymentEndTime = endTime
}

// Ensure that Machine implements Machiner
var _ Machiner = (*Machine)(nil)
