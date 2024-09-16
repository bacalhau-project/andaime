package models

import (
	"fmt"
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
	DeploymentTypeGCP     DeploymentType = "GCP"
)

type Deployment struct {
	mu                     sync.RWMutex
	Name                   string
	DeploymentType         DeploymentType
	Azure                  *AzureConfig
	GCP                    *GCPConfig
	Machines               map[string]Machiner // Change *Machiner to Machiner
	UniqueID               string
	StartTime              time.Time
	EndTime                time.Time
	ProjectID              string
	Locations              []string
	AllowedPorts           []int
	SSHUser                string
	SSHPort                int
	SSHPublicKeyPath       string
	SSHPublicKeyMaterial   string
	SSHPrivateKeyPath      string
	SSHPrivateKeyMaterial  string
	OrchestratorIP         string
	Tags                   map[string]*string
	ProjectServiceAccounts map[string]ServiceAccountInfo
	deploymentMutex        sync.RWMutex
}

func NewDeployment() (*Deployment, error) {
	projectPrefix := viper.GetString("general.project_prefix")
	if projectPrefix == "" {
		return nil, fmt.Errorf("general.project_prefix is not set")
	}
	uniqueID := time.Now().Format("0601021504")
	projectID := projectPrefix + "-" + uniqueID
	deployment := &Deployment{
		StartTime:              time.Now(),
		Machines:               make(map[string]Machiner), // Change *Machine to Machiner
		UniqueID:               uniqueID,
		Azure:                  &AzureConfig{},
		GCP:                    &GCPConfig{},
		ProjectID:              projectID,
		Tags:                   make(map[string]*string),
		ProjectServiceAccounts: make(map[string]ServiceAccountInfo),
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
		"ProjectID":             d.ProjectID,
		"UniqueID":              d.UniqueID,
	}
}

func (d *Deployment) UpdateViperConfig() error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v := viper.GetViper()
	deploymentPath := fmt.Sprintf("deployments.azure.%s", d.Azure.ResourceGroupName)
	viperMachines := make(map[string]map[string]interface{})
	for _, machine := range d.Machines {
		viperMachines[machine.GetName()] = map[string]interface{}{
			"Name":         machine.GetName(),
			"PublicIP":     machine.GetPublicIP(),
			"PrivateIP":    machine.GetPrivateIP(),
			"Orchestrator": machine.IsOrchestrator(),
		}
	}

	v.Set(deploymentPath, viperMachines)
	return v.WriteConfig()
}

func (d *Deployment) GetMachine(name string) Machiner {
	d.deploymentMutex.RLock()
	defer d.deploymentMutex.RUnlock()
	if machine, ok := d.Machines[name]; ok {
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
		machines[name] = machine
	}
	return machines
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
	Tags                  map[string]*string
}

type GCPConfig struct {
	ProjectID              string
	OrganizationID         string
	Region                 string
	Zone                   string
	DefaultMachineType     string
	DefaultDiskSizeGB      int32
	BillingAccountID       string
	ServiceAccountEmail    string
	ProjectServiceAccounts map[string]ServiceAccountInfo
	Tags                   map[string]*string
}
