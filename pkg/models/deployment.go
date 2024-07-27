package models

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/viper"
)

type Machine struct {
	ID                   string
	Name                 string
	Location             string
	Parameters           []Parameters
	PublicIP             *armnetwork.PublicIPAddress
	NetworkSecurityGroup *armnetwork.SecurityGroup
	Interface            *armnetwork.Interface
	VMSize               string
	DiskSizeGB           int32 `default:"30"`
	ComputerName         string
}

type Parameters struct {
	Count        int
	Type         string
	Orchestrator bool
}

type Deployment struct {
	Name                  string
	ResourceGroupName     string
	ResourceGroupLocation string
	Locations             []string
	OrchestratorNode      *Machine
	Machines              []Machine
	Subnets               map[string][]*armnetwork.Subnet
	NetworkSecurityGroups map[string]*armnetwork.SecurityGroup
	ProjectID             string
	UniqueID              string
	Tags                  map[string]*string
	AllowedPorts          []int
	SSHPublicKeyPath      string
	SSHPrivateKeyPath     string
	DefaultVMSize         string `default:"Standard_B2s"`
	DefaultDiskSizeGB     int32  `default:"30"`
	DefaultLocation       string `default:"eastus"`
}

func (d *Deployment) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"ResourceGroupName":     d.ResourceGroupName,
		"ResourceGroupLocation": d.ResourceGroupLocation,
		"OrchestratorNode":      d.OrchestratorNode,
		"Machines":              d.Machines,
		"VNet":                  d.Subnets,
		"ProjectID":             d.ProjectID,
		"UniqueID":              d.UniqueID,
		"Tags":                  d.Tags,
	}
}

// UpdateViperConfig updates the Viper configuration with the current Deployment state
func (d *Deployment) UpdateViperConfig() error {
	v := viper.GetViper()
	deploymentPath := fmt.Sprintf("deployments.azure.%s", d.ResourceGroupName)
	v.Set(deploymentPath, d.ToMap())
	return v.WriteConfig()
}

func (d *Deployment) SetSubnet(location string, subnets ...*armnetwork.Subnet) {
	if d.Subnets == nil {
		d.Subnets = make(map[string][]*armnetwork.Subnet)
	}
	d.Subnets[location] = append(d.Subnets[location], subnets...)
}
