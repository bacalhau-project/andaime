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
	ResourceType  ResourceTypes
	ResourceState ResourceState
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

// Add this new type
type DeploymentType string

const (
	DeploymentTypeAzure DeploymentType = "Azure"
	DeploymentTypeGCP   DeploymentType = "GCP"
	// Add other cloud providers as needed
)

type Deployment struct {
	mu   sync.RWMutex
	Name string

	// Azure specific
	ResourceGroupName     string
	ResourceGroupLocation string

	// GCP specific
	OrganizationID string

	// ProjectID is the project ID for the deployment - works on multiple clouds
	ProjectID string

	// BillingAccountID is the billing account ID for the deployment - works on GCP
	BillingAccountID string

	// ServiceAccountEmail is the email of the service account for the deployment - works on GCP
	ServiceAccountEmail    string
	ProjectServiceAccounts map[string]ServiceAccountInfo

	Locations      []string
	OrchestratorIP string
	Machines       map[string]*Machine
	UniqueID       string

	Tags   map[string]*string
	Labels map[string]string

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

	Cleanup func()
	Type    DeploymentType
}

func NewDeployment(deploymentType DeploymentType) (*Deployment, error) {
	deployment := &Deployment{
		StartTime: time.Now(),
		Machines:  make(map[string]*Machine),
		Tags:      make(map[string]*string),
		ProjectID: viper.GetString("general.project_prefix"),
		UniqueID:  time.Now().Format("060102150405"),
		Type:      deploymentType,
	}
	return deployment, nil
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
func (d *Deployment) CreateMachine(name string) {
	d.deploymentMutex.Lock()
	defer d.deploymentMutex.Unlock()
	d.Machines[name] = &Machine{}
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
