package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/aws/types"
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

// Use ServiceType from types.go
var (
	RequiredServices = []ServiceType{
		ServiceTypeSSH,          // Use existing ServiceType constants from types.go
		ServiceTypeDocker,
		ServiceTypeCorePackages,
		ServiceTypeBacalhau,
		ServiceTypeScript,
	}
)

type MachineResource struct {
	ResourceName  string
	ResourceType  ResourceType
	ResourceState MachineResourceState
	ResourceValue string
}

type Parameters struct {
	Count        int    `json:"count" yaml:"count"`
	Type         string `json:"type" yaml:"type"`
	Orchestrator bool   `json:"orchestrator" yaml:"orchestrator"`
	Spot         bool   `json:"spot,omitempty" yaml:"spot,omitempty"`
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
		Tags:                   make(map[string]string),
		ProjectServiceAccounts: make(map[string]ServiceAccountInfo),
	}
	deployment.InitializeAWSDeployment()

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
		deployment.SetProjectID(uniqueProjectID)
	}

	return deployment, nil
}

func (d *Deployment) InitializeAWSDeployment() *AWSDeployment {
	if d.AWS == nil {
		d.AWS = &AWSDeployment{}
		d.AWS.RegionalResources = &RegionalResources{}
		d.AWS.RegionalResources.VPCs = make(map[string]*AWSVPC)
		d.AWS.RegionalResources.Clients = make(map[string]aws_interface.EC2Clienter)
	}

	return d.AWS
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
					machineRegion := machine.GetRegion()
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
	r.RLock()
	defer r.RUnlock()
	if r.VPCs == nil {
		r.VPCs = make(map[string]*AWSVPC)
	}
	return r.VPCs[region]
}

func (r *RegionalResources) UpdateVPC(
	deployment *Deployment,
	region string,
	updateFn func(*AWSVPC) error,
) error {
	vpc := r.GetVPC(region)
	if vpc == nil {
		return fmt.Errorf("VPC not found for region %s", region)
	}
	return updateFn(vpc)
}

func (r *RegionalResources) SetVPC(region string, vpc *AWSVPC) {
	r.Lock()
	defer r.Unlock()
	if r.VPCs == nil {
		r.VPCs = make(map[string]*AWSVPC)
	}
	r.VPCs[region] = vpc
}

func (r *RegionalResources) GetClient(region string) aws_interface.EC2Clienter {
	r.RLock()
	defer r.RUnlock()
	return r.Clients[region]
}

func (r *RegionalResources) SetClient(region string, client aws_interface.EC2Clienter) {
	r.Lock()
	defer r.Unlock()
	r.Clients[region] = client
}

func (r *RegionalResources) SetSGID(region string, sgID string) {
	r.Lock()
	defer r.Unlock()
	if r.VPCs == nil {
		r.VPCs = make(map[string]*AWSVPC)
	}
	if r.VPCs[region] == nil {
		r.VPCs[region] = &AWSVPC{}
	}
	r.VPCs[region].SecurityGroupID = sgID
}

func (r *RegionalResources) GetRegions() []string {
	r.RLock()
	defer r.RUnlock()
	regions := []string{}
	for region := range r.VPCs {
		regions = append(regions, region)
	}
	return regions
}

// SaveVPCConfig saves VPC configuration to viper
func (rm *RegionalResources) SaveVPCConfig(
	deployment *Deployment,
	region string,
	vpc *AWSVPC,
) error {
	deploymentPath := fmt.Sprintf("deployments.%s", deployment.UniqueID)
	viper.Set(fmt.Sprintf("%s.aws.regions.%s.vpc_id", deploymentPath, region), vpc.VPCID)

	return viper.WriteConfig()
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
