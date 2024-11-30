package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"

	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
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
	ServiceTypeSSH          = ServiceType{Name: "SSH", State: ServiceStateNotStarted}
	ServiceTypeDocker       = ServiceType{Name: "Docker", State: ServiceStateNotStarted}
	ServiceTypeBacalhau     = ServiceType{Name: "Bacalhau", State: ServiceStateNotStarted}
	ServiceTypeCorePackages = ServiceType{Name: "CorePackages", State: ServiceStateNotStarted}
	ServiceTypeScript       = ServiceType{Name: "Script", State: ServiceStateNotStarted}
)

type MachineResource struct {
	ResourceName  string
	ResourceType  ResourceType
	ResourceState MachineResourceState
	ResourceValue string
}

type Parameters struct {
	Count        int
	Type         string
	Orchestrator bool
}

type ServiceAccountInfo struct {
	Email string
	Key   string
}

type Disk struct {
	Name   string
	ID     string
	SizeGB int32
	State  armcompute.DiskState
}

type DeploymentType string

const (
	DeploymentTypeUnknown DeploymentType = "Unknown"
	DeploymentTypeAzure   DeploymentType = "Azure"
	DeploymentTypeAWS     DeploymentType = "AWS"
	DeploymentTypeGCP     DeploymentType = "GCP"
)

type Deployment struct {
	mu                     sync.RWMutex
	Name                   string
	ViperPath              string
	DeploymentType         DeploymentType
	Azure                  *AzureConfig
	GCP                    *GCPConfig
	AWS                    *AWSDeployment
	Machines               map[string]Machiner
	UniqueID               string
	StartTime              time.Time
	EndTime                time.Time
	projectID              string
	Locations              []string
	AllowedPorts           []int
	SSHUser                string
	SSHPort                int
	SSHPublicKeyPath       string
	SSHPublicKeyMaterial   string
	SSHPrivateKeyPath      string
	SSHPrivateKeyMaterial  string
	SSHKeyName             string
	OrchestratorIP         string
	Tags                   map[string]string
	ProjectServiceAccounts map[string]ServiceAccountInfo
	deploymentMutex        sync.RWMutex
	BacalhauSettings       []BacalhauSettings
	CustomScriptPath       string
}

type DeploymentStatus string

const (
	DeploymentStatusUnknown    DeploymentStatus = "Unknown"
	DeploymentStatusNotStarted DeploymentStatus = "NotStarted"
	DeploymentStatusInProgress DeploymentStatus = "InProgress"
	DeploymentStatusSucceeded  DeploymentStatus = "Succeeded"
	DeploymentStatusFailed     DeploymentStatus = "Failed"
)

func NewDeployment() (*Deployment, error) {
	l := logger.Get()
	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		projectPrefix = "andaime"
	}
	uniqueID := fmt.Sprintf("u%s", time.Now().Format("0601021504"))
	deployment := &Deployment{
		StartTime:              time.Now(),
		Machines:               make(map[string]Machiner), // Change *Machine to Machiner
		UniqueID:               uniqueID,
		Azure:                  &AzureConfig{},
		GCP:                    &GCPConfig{},
		AWS:                    &AWSDeployment{},
		Tags:                   make(map[string]string),
		ProjectServiceAccounts: make(map[string]ServiceAccountInfo),
	}

	timestamp := time.Now().Format("01021504") // mmddhhmm
	if deployment.DeploymentType == DeploymentTypeGCP {
		uniqueProjectID := fmt.Sprintf("%s-%s", projectPrefix, timestamp)

		// Check if the unique project ID is too long
		if len(uniqueProjectID) > MaximumGCPUniqueProjectIDLength {
			l.Warnf(
				"unique project ID is too long, it should be less than %d characters -- %s...",
				MaximumGCPUniqueProjectIDLength,
				uniqueProjectID[:MaximumGCPUniqueProjectIDLength],
			)
			return nil, fmt.Errorf(
				"unique project ID is too long, it should be less than %d characters",
				MaximumGCPUniqueProjectIDLength,
			)
		}

		l.Debugf("Ensuring project: %s", uniqueProjectID)

		deployment.SetProjectID(uniqueProjectID)
	}

	return deployment, nil
}

func (d *Deployment) ToMap() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return map[string]interface{}{
		"ResourceGroupName":     d.Azure.ResourceGroupName,
		"ResourceGroupLocation": d.Azure.ResourceGroupLocation,
		"Machines":              d.Machines,
		"ProjectID":             d.projectID,
		"UniqueID":              d.UniqueID,
	}
}

func (d *Deployment) UpdateViperConfig() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	deploymentPath := fmt.Sprintf("deployments.%s", d.UniqueID)

	// Common deployment settings
	viper.Set(fmt.Sprintf("%s.provider", deploymentPath), strings.ToLower(string(d.DeploymentType)))

	// Provider-specific settings
	switch d.DeploymentType {
	case DeploymentTypeAzure:
		viper.Set(fmt.Sprintf("%s.azure.subscription_id", deploymentPath), d.Azure.SubscriptionID)
		viper.Set(fmt.Sprintf("%s.azure.resource_group", deploymentPath), d.Azure.ResourceGroupName)
		for _, machine := range d.Machines {
			viper.Set(
				fmt.Sprintf("%s.azure.machines.%s", deploymentPath, machine.GetName()),
				map[string]interface{}{
					"name":         machine.GetName(),
					"public_ip":    machine.GetPublicIP(),
					"private_ip":   machine.GetPrivateIP(),
					"orchestrator": machine.IsOrchestrator(),
				},
			)
		}

	case DeploymentTypeGCP:
		viper.Set(fmt.Sprintf("%s.gcp.project_id", deploymentPath), d.GCP.ProjectID)
		viper.Set(fmt.Sprintf("%s.gcp.organization_id", deploymentPath), d.GCP.OrganizationID)
		for _, machine := range d.Machines {
			viper.Set(
				fmt.Sprintf("%s.gcp.machines.%s", deploymentPath, machine.GetName()),
				map[string]interface{}{
					"name":         machine.GetName(),
					"public_ip":    machine.GetPublicIP(),
					"private_ip":   machine.GetPrivateIP(),
					"orchestrator": machine.IsOrchestrator(),
				},
			)
		}

	case DeploymentTypeAWS:
		viper.Set(fmt.Sprintf("%s.aws.account_id", deploymentPath), d.AWS.AccountID)
		if d.AWS.RegionalResources != nil {
			for region, vpc := range d.AWS.RegionalResources.VPCs {
				regionPath := fmt.Sprintf("%s.aws.regions.%s", deploymentPath, region)
				viper.Set(fmt.Sprintf("%s.vpc_id", regionPath), vpc.VPCID)
				viper.Set(fmt.Sprintf("%s.security_group_id", regionPath), vpc.SecurityGroupID)

				// Group machines by region
				for _, machine := range d.Machines {
					machineRegion := machine.GetLocation()
					if machineRegion == "" {
						continue
					}
					// Convert AWS Zone to region if necessary (e.g., us-west-2a -> us-west-2)
					if len(machineRegion) > 0 &&
						machineRegion[len(machineRegion)-1] >= 'a' &&
						machineRegion[len(machineRegion)-1] <= 'z' {
						machineRegion = machineRegion[:len(machineRegion)-1]
					}
					if machineRegion == region {
						viper.Set(
							fmt.Sprintf("%s.machines.%s", regionPath, machine.GetName()),
							map[string]interface{}{
								"name":         machine.GetName(),
								"public_ip":    machine.GetPublicIP(),
								"private_ip":   machine.GetPrivateIP(),
								"orchestrator": machine.IsOrchestrator(),
							},
						)
					}
				}
			}
		}
	}

	return viper.WriteConfig()
}

func (d *Deployment) GetMachine(name string) Machiner {
	d.deploymentMutex.RLock()
	defer d.deploymentMutex.RUnlock()
	if machine, ok := d.Machines[name]; ok && !machine.IsFailed() {
		return machine
	}
	return nil
}

func (d *Deployment) CreateMachine(name string, machine Machiner) {
	d.deploymentMutex.Lock()
	defer d.deploymentMutex.Unlock()
	d.Machines[name] = machine
}

func (d *Deployment) UpdateMachine(name string, updater func(Machiner)) error {
	d.deploymentMutex.Lock()
	defer d.deploymentMutex.Unlock()
	if machine, ok := d.Machines[name]; ok {
		updater(machine)
		return nil
	}
	return fmt.Errorf("machine %s not found", name)
}

func (d *Deployment) SetMachine(name string, machine Machiner) {
	d.deploymentMutex.Lock()
	defer d.deploymentMutex.Unlock()
	d.Machines[name] = machine
}

func (d *Deployment) SetMachines(machines map[string]Machiner) {
	d.Machines = make(map[string]Machiner)
	for name, machine := range machines {
		d.SetMachine(name, machine)
	}
}

func (d *Deployment) GetMachines() map[string]Machiner {
	d.deploymentMutex.RLock()
	defer d.deploymentMutex.RUnlock()
	machines := make(map[string]Machiner)
	for name, machine := range d.Machines {
		if !machine.IsFailed() {
			machines[name] = machine
		}
	}
	return machines
}

func (d *Deployment) SetLocations(locations map[string]bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Locations == nil {
		d.Locations = []string{}
	}

	// Should be of the form map[location]bool
	for location := range locations {
		d.Locations = append(d.Locations, location)
	}
}

func (d *Deployment) GetProjectID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.projectID == "" {
		panic("project ID is not set in the deployment")
	}
	return d.projectID
}

func (d *Deployment) GetSSHKeyName() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.SSHKeyName
}

func (d *Deployment) SetProjectID(projectID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if projectID == "" {
		return
	}
	d.projectID = projectID
	d.GCP.ProjectID = projectID
}

type StatusUpdateMsg struct {
	Status *DisplayStatus
}

func (d *Deployment) GetCloudResources(cloudType DeploymentType) interface{} {
	switch cloudType {
	case DeploymentTypeAzure:
		return d.Azure
	case DeploymentTypeGCP:
		return d.GCP
	default:
		return nil
	}
}

func (d *Deployment) GetCloudConfig(cloudType DeploymentType) interface{} {
	switch cloudType {
	case DeploymentTypeAzure:
		return d.Azure
	case DeploymentTypeGCP:
		return d.GCP
	default:
		return nil
	}
}

type AzureConfig struct {
	ResourceGroupName     string
	ResourceGroupLocation string
	SubscriptionID        string
	DefaultVMSize         string
	DefaultDiskSizeGB     int32
	DefaultLocation       string
	Tags                  map[string]string
	DefaultCountPerZone   int
}

type GCPConfig struct {
	ProjectID              string
	OrganizationID         string
	DefaultRegion          string
	DefaultZone            string
	DefaultMachineType     string
	DefaultDiskSizeGB      int32
	BillingAccountID       string
	ServiceAccountEmail    string
	ProjectServiceAccounts map[string]ServiceAccountInfo
	Tags                   map[string]string
	DefaultCountPerZone    int
}

type AWSConfig struct {
	AccountID             string
	DefaultMachineType    string
	DefaultDiskSizeGB     int32
	DefaultCountPerRegion int
	RegionalResources     *RegionalResources
}

// RegionalResources tracks VPCs and other resources per region
type RegionalResources struct {
	mu      sync.RWMutex
	VPCs    map[string]*AWSVPC // key is region
	Clients map[string]aws_interface.EC2Clienter
}

func (r *RegionalResources) Lock() {
	if r == nil {
		return
	}
	r.mu.Lock()
}

func (r *RegionalResources) Unlock() {
	if r == nil {
		return
	}
	r.mu.Unlock()
}

func (r *RegionalResources) RLock() {
	r.mu.Lock()
}

func (r *RegionalResources) RUnlock() {
	r.mu.Unlock()
}

func (r *RegionalResources) GetVPC(region string) *AWSVPC {
	r.Lock()
	defer r.Unlock()
	if r.VPCs == nil {
		r.VPCs = make(map[string]*AWSVPC)
	}
	return r.VPCs[region]
}

func (r *RegionalResources) SetVPC(region string, vpc *AWSVPC) {
	r.Lock()
	defer r.Unlock()
	if r.VPCs == nil {
		r.VPCs = make(map[string]*AWSVPC)
	}
	r.VPCs[region] = vpc
}

type RegionalVPC struct {
	VPCID              string
	Region             string
	SecurityGroupID    string
	PublicSubnetIDs    []string
	PrivateSubnetIDs   []string
	InternetGatewayID  string
	PublicRouteTableID string
}
