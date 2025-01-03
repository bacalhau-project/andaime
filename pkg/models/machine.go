package models

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	internal_aws "github.com/bacalhau-project/andaime/internal/clouds/aws"
	internal_azure "github.com/bacalhau-project/andaime/internal/clouds/azure"
	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

var ExpectedDockerHelloWorldCommand = "sudo docker run hello-world"
var ExpectedDockerOutput = "Hello from Docker!"

type Machiner interface {
	// Basic information
	GetID() string
	GetName() string
	SetName(string)
	GetType() ResourceType
	GetVMSize() string
	GetDiskSizeGB() int
	SetDiskSizeGB(size int)
	IsFailed() bool
	SetFailed(bool)
	GetDiskImageFamily() string
	SetDiskImageFamily(family string)
	GetDiskImageURL() string
	SetDiskImageURL(url string)
	GetImageID() string
	SetImageID(id string)
	GetDisplayLocation() string
	SetDisplayLocation(location string) error
	GetRegion() string
	SetRegion(region string)
	GetZone() string
	SetZone(zone string)
	GetPublicIP() string
	SetPublicIP(ip string)
	GetPrivateIP() string
	SetPrivateIP(ip string)

	// Stage management
	GetStage() ProvisioningStage
	SetStage(stage ProvisioningStage)

	// AWS Spot Market Options
	GetSpotMarketOptions() *ec2Types.InstanceMarketOptionsRequest
	SetSpotMarketOptions(options *ec2Types.InstanceMarketOptionsRequest)

	// SSH related
	GetSSHUser() string
	SetSSHUser(user string)
	GetSSHPublicKeyMaterial() []byte
	SetSSHPublicKeyMaterial([]byte)
	GetSSHPublicKeyPath() string
	SetSSHPublicKeyPath(path string)
	GetSSHPrivateKeyMaterial() []byte
	SetSSHPrivateKeyMaterial([]byte)
	GetSSHPrivateKeyPath() string
	SetSSHPrivateKeyPath(path string)
	GetSSHPort() int
	SetSSHPort(port int)

	// Status and state
	GetNodeType() string
	SetNodeType(nodeType string)
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
	GetProvisioningStage() ProvisioningStage
	SetProvisioningStage(stage ProvisioningStage)
	ResourcesComplete() (int, int)

	// Service management
	GetServices() map[string]ServiceType
	GetServiceState(serviceName string) ServiceState
	SetServiceState(serviceName string, state ServiceState)
	SSHEnabled() bool
	DockerEnabled() bool
	BacalhauEnabled() bool
	CustomScriptEnabled() bool
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

	// Machine configuration
	GetParameters() Parameters
	GetMachineType() string
}

const (
	BacalhauNodeTypeCompute      = "compute"
	BacalhauNodeTypeOrchestrator = "requester"
)

type Machine struct {
	ID              string
	Name            string
	Type            ResourceType
	DisplayLocation string
	Region          string
	Zone            string
	NodeType        string
	Stage           ProvisioningStage

	StatusMessage string
	Parameters    Parameters
	PublicIP      string
	PrivateIP     string
	StartTime     time.Time
	Failed        bool

	VMSize          string
	DiskSizeGB      int
	DiskImageFamily string
	DiskImageURL    string
	ImageID         string
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

	CustomScriptExecuted bool

	machineResources  map[string]MachineResource
	ProvisioningStage ProvisioningStage
	machineServices   map[string]ServiceType

	stateMutex sync.RWMutex
	done       bool

	DeploymentType DeploymentType
	CloudProvider  DeploymentType
	CloudSpecific  CloudSpecificInfo
}

type CloudSpecificInfo struct {
	// Azure specific
	ResourceGroupName string

	// AWS specific
	SpotMarketOptions *ec2Types.InstanceMarketOptionsRequest
}

func NewMachine(
	cloudProvider DeploymentType,
	location string,
	vmSize string,
	diskSizeGB int,
	region string,
	zone string,
	cloudSpecificInfo CloudSpecificInfo,
) (Machiner, error) {
	newID := utils.CreateShortID()

	if !IsValidLocation(cloudProvider, location) {
		if location == "" {
			location = "<NONE PROVIDED>"
		}
		return nil, fmt.Errorf("invalid location for %s: %s", cloudProvider, location)
	}

	machineName := fmt.Sprintf("%s-vm", newID)
	returnMachine := &Machine{
		ID:              machineName,
		Name:            machineName,
		StartTime:       time.Now(),
		VMSize:          vmSize,
		DiskSizeGB:      diskSizeGB,
		CloudProvider:   cloudProvider,
		CloudSpecific:   cloudSpecificInfo,
		Stage:           StageVMRequested, // Initialize stage as VM requested
		machineResources: make(map[string]MachineResource),
	}

	// There is some logic in SetLocation so we do this outside of the struct
	err := returnMachine.SetDisplayLocation(location)
	if err != nil {
		return nil, err
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
	} else if cloudProvider == DeploymentTypeAWS {
		for _, resource := range RequiredAWSResources {
			returnMachine.SetMachineResource(
				resource.GetResourceLowerString(),
				ResourceStateNotStarted,
			)
		}
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

func (mach *Machine) SetDisplayLocation(location string) error {
	l := logger.Get()
	region, zone, err := general.NormalizeLocation(string(mach.CloudProvider), location)
	if err != nil {
		return err
	}
	l.Debugf("Normalized location: %s -> %s, %s", location, region, zone)

	switch mach.CloudProvider {
	case DeploymentTypeAzure:
		if !internal_azure.IsValidAzureRegion(region) {
			return fmt.Errorf("invalid Azure location: %s", location)
		}
		mach.Region = region
		mach.Zone = zone
	case DeploymentTypeGCP:
		if !internal_gcp.IsValidGCPZone(zone) {
			return fmt.Errorf("invalid GCP location: %s", location)
		}
		mach.Region = region
		mach.Zone = zone
	case DeploymentTypeAWS:
		if !internal_aws.IsValidAWSRegion(region) {
			return fmt.Errorf("invalid AWS region: %s", region)
		}
		mach.Region = region
		mach.Zone = zone
	default:
		return fmt.Errorf("unknown deployment type: %s", mach.DeploymentType)
	}
	mach.DisplayLocation = location
	return nil
}

func (mach *Machine) SSHEnabled() bool {
	return mach.GetServiceState(ServiceTypeSSH.Name) == ServiceStateSucceeded
}

func (mach *Machine) DockerEnabled() bool {
	return mach.GetServiceState(ServiceTypeDocker.Name) == ServiceStateSucceeded
}

func (mach *Machine) BacalhauEnabled() bool {
	return mach.GetServiceState(ServiceTypeBacalhau.Name) == ServiceStateSucceeded
}

func (mach *Machine) CustomScriptEnabled() bool {
	return mach.GetServiceState(ServiceTypeScript.Name) == ServiceStateSucceeded
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
	bothTrue := resourcesComplete && servicesComplete
	return bothTrue
}

func (mach *Machine) SetCustomScriptExecuted(executed bool) {
	mach.CustomScriptExecuted = executed
}

func (mach *Machine) IsCustomScriptExecuted() bool {
	return mach.CustomScriptExecuted
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

	var requiredResources []ResourceType
	if mach.CloudProvider == DeploymentTypeAzure {
		requiredResources = RequiredAzureResources
	} else if mach.CloudProvider == DeploymentTypeGCP {
		requiredResources = RequiredGCPResources
	} else if mach.CloudProvider == DeploymentTypeAWS {
		requiredResources = RequiredAWSResources
	} else {
		l.Errorf("Unknown cloud provider: %s", mach.CloudProvider)
		return 0, 0
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
		return service.GetState()
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
		service.SetState(state)
		mach.machineServices[serviceName] = service
	} else {
		mach.machineServices[serviceName] = NewServiceType(serviceName, state)
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

	mach.SetServiceState(ServiceTypeDocker.Name, ServiceStateUpdating)
	if err := mach.installDocker(ctx); err != nil {
		mach.SetServiceState(ServiceTypeDocker.Name, ServiceStateFailed)
		return err
	}

	if err := mach.verifyDocker(ctx); err != nil {
		mach.SetServiceState(ServiceTypeDocker.Name, ServiceStateFailed)
		return err
	}
	mach.SetServiceState(ServiceTypeDocker.Name, ServiceStateSucceeded)

	mach.SetServiceState(ServiceTypeCorePackages.Name, ServiceStateUpdating)

	if err := mach.installCorePackages(ctx); err != nil {
		mach.SetServiceState(ServiceTypeCorePackages.Name, ServiceStateFailed)
		return err
	}
	mach.SetServiceState(ServiceTypeCorePackages.Name, ServiceStateSucceeded)

	return nil
}

func (mach *Machine) installDocker(ctx context.Context) error {
	return mach.installService(
		ctx,
		ServiceTypeDocker.Name,
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

	output, err := sshConfig.ExecuteCommand(ctx, ExpectedDockerHelloWorldCommand)
	if err != nil {
		l.Errorf("Failed to verify Docker on machine %s: %v", mach.Name, err)
		return err
	}

	if !strings.Contains(output, ExpectedDockerOutput) {
		return fmt.Errorf(
			"docker verify ran, but did not get expected output on machine %s: output: %s",
			mach.Name,
			output,
		)
	}

	return nil
}

func (mach *Machine) installCorePackages(ctx context.Context) error {
	return mach.installService(
		ctx,
		ServiceTypeCorePackages.Name,
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

	// Connect to the remote host
	if _, err := sshConfig.Connect(); err != nil {
		l.Errorf("Error connecting to remote host: %v", err)
		return err
	}
	defer sshConfig.Close()

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

// For Azure, "location" is the region.
// For GCP, "location" is the zone.
// For AWS, "location" is the zone.
func IsValidLocation(deploymentType DeploymentType, location string) bool {
	region, zone, err := general.NormalizeLocation(
		string(deploymentType),
		location,
	)
	if err != nil {
		return false
	}

	switch deploymentType {
	case DeploymentTypeAzure:
		return internal_azure.IsValidAzureRegion(region)
	case DeploymentTypeGCP:
		return internal_gcp.IsValidGCPZone(zone)
	case DeploymentTypeAWS:
		return internal_aws.IsValidAWSRegion(region)
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

func (mach *Machine) SetName(name string) {
	mach.Name = name
}

func (mach *Machine) IsFailed() bool {
	return mach.Failed
}

func (mach *Machine) SetFailed(failed bool) {
	mach.Failed = failed
}

func (mach *Machine) GetType() ResourceType {
	return mach.Type
}

func (mach *Machine) GetRegion() string {
	return mach.Region
}

func (mach *Machine) SetRegion(region string) {
	mach.Region = region
}

func (mach *Machine) GetZone() string {
	return mach.Zone
}

func (mach *Machine) SetZone(zone string) {
	mach.Zone = zone
}

func (mach *Machine) GetDisplayLocation() string {
	return mach.DisplayLocation
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

// GetStage returns the current provisioning stage with mutex protection
func (mach *Machine) GetStage() ProvisioningStage {
	mach.stateMutex.RLock()
	defer mach.stateMutex.RUnlock()
	return mach.Stage
}

// SetStage updates the current provisioning stage with mutex protection
func (mach *Machine) SetStage(stage ProvisioningStage) {
	mach.stateMutex.Lock()
	defer mach.stateMutex.Unlock()
	mach.Stage = stage
}

func (mach *Machine) SetStatusMessage(message string) {
	mach.StatusMessage = message
}

func (mach *Machine) GetStartTime() time.Time {
	if mach.StartTime.IsZero() {
		return time.Now()
	}
	return mach.StartTime
}

func (mach *Machine) GetDeploymentEndTime() time.Time {
	if mach.DeploymentEndTime.IsZero() {
		return time.Now()
	}
	return mach.DeploymentEndTime
}

func (mach *Machine) GetDiskImageFamily() string {
	return mach.DiskImageFamily
}

func (mach *Machine) SetDiskImageFamily(family string) {
	mach.DiskImageFamily = family
}

func (mach *Machine) GetSpotMarketOptions() *ec2Types.InstanceMarketOptionsRequest {
	if mach.CloudSpecific == (CloudSpecificInfo{}) {
		return nil
	}
	return mach.CloudSpecific.SpotMarketOptions
}

func (mach *Machine) SetSpotMarketOptions(options *ec2Types.InstanceMarketOptionsRequest) {
	if mach.CloudSpecific == (CloudSpecificInfo{}) {
		mach.CloudSpecific = CloudSpecificInfo{}
	}
	mach.CloudSpecific.SpotMarketOptions = options
}

func (mach *Machine) GetImageID() string {
	return mach.ImageID
}

func (mach *Machine) SetImageID(imageID string) {
	mach.ImageID = imageID
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

func (mach *Machine) GetNodeType() string {
	return mach.NodeType
}

func (mach *Machine) SetNodeType(nodeType string) {
	mach.NodeType = nodeType
}

// GetParameters returns the machine parameters
func (mach *Machine) GetParameters() Parameters {
	return mach.Parameters
}

// GetMachineType returns the machine type (VM size)
func (mach *Machine) GetMachineType() string {
	return mach.VMSize
}

func MachineConfigToWrite(machine Machiner) map[string]interface{} {
	sshEnabled := machine.GetServiceState(
		ServiceTypeSSH.Name,
	) == ServiceStateSucceeded
	dockerInstalled := machine.GetServiceState(
		ServiceTypeDocker.Name,
	) == ServiceStateSucceeded
	bacalhauInstalled := machine.GetServiceState(
		ServiceTypeBacalhau.Name,
	) == ServiceStateSucceeded
	scriptInstalled := machine.GetServiceState(
		ServiceTypeScript.Name,
	) == ServiceStateSucceeded
	orchestrator := machine.IsOrchestrator()

	return map[string]interface{}{
		"name":              machine.GetName(),
		"publicip":          machine.GetPublicIP(),
		"privateip":         machine.GetPrivateIP(),
		"SSHEnabled":        sshEnabled,
		"DockerInstalled":   dockerInstalled,
		"BacalhauInstalled": bacalhauInstalled,
		"ScriptInstalled":   scriptInstalled,
		"Orchestrator":      orchestrator,
	}
}

func (mach *Machine) GetProvisioningStage() ProvisioningStage {
	return mach.ProvisioningStage
}

func (mach *Machine) SetProvisioningStage(stage ProvisioningStage) {
	mach.ProvisioningStage = stage
}

// Ensure that Machine implements Machiner
var _ Machiner = (*Machine)(nil)
