package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
)

var (
	basePriority = 100
)

// createPublicIP creates a public IP address
func (p *AzureProvider) CreatePublicIP(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.PublicIPAddress, error) {
	createdIP, err := p.Client.CreatePublicIP(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		machine.ID,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP: %v", err)
	}

	return &createdIP, nil
}

// createNIC creates a network interface with both public and private IP
func (p *AzureProvider) CreateNIC(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.Interface, error) {
	if deployment.Subnets == nil {
		return nil, fmt.Errorf("subnets not found")
	}

	if machine.Location == "" {
		return nil, fmt.Errorf("location not found")
	}

	if len(deployment.Subnets[machine.Location]) == 0 {
		return nil, fmt.Errorf("no subnets found for location %s", machine.Location)
	}

	if machine.NetworkSecurityGroup == nil {
		return nil, fmt.Errorf("network security group not found")
	}

	if machine.NetworkSecurityGroup.ID == nil {
		return nil, fmt.Errorf("network security group ID not found")
	}

	if len(deployment.Subnets[machine.Location]) == 0 {
		return nil, fmt.Errorf("no subnets found for location %s", machine.Location)
	}

	createdNIC, err := p.Client.CreateNetworkInterface(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		getNetworkInterfaceName(machine.ID),
		deployment.Tags,
		deployment.Subnets[machine.Location][0],
		machine.PublicIP,
		machine.NetworkSecurityGroup,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	return &createdNIC, nil
}

// createVM creates the virtual machine
func (p *AzureProvider) CreateVirtualMachine(
	ctx context.Context,
	deployment *models.Deployment,
	machine models.Machine,
	disp *display.Display,
) (*armcompute.VirtualMachine, error) {
	if machine.Interface == nil {
		return nil, fmt.Errorf("network interface not created for machine %s", machine.ID)
	}

	params := armcompute.VirtualMachine{
		Location: to.Ptr(machine.Location),
		Tags:     deployment.Tags,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(machine.VMSize)),
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(machine.Name),
				AdminUsername: to.Ptr("azureuser"),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr("/home/azureuser/.ssh/authorized_keys"),
								KeyData: to.Ptr(deployment.SSHPublicKeyPath),
							},
						},
					},
				},
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr("Canonical"),
					Offer:     to.Ptr("UbuntuServer"),
					SKU:       to.Ptr("18.04-LTS"),
					Version:   to.Ptr("latest"),
				},
				OSDisk: &armcompute.OSDisk{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
					DiskSizeGB: to.Ptr(machine.DiskSizeGB),
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(*machine.Interface.ID),
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							Primary: to.Ptr(true),
						},
					},
				},
			},
		},
	}

	createdVM, err := p.Client.CreateVirtualMachine(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		deployment.UniqueID,
		&params,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual machine: %v", err)
	}

	return &createdVM, nil
}

// createNSG creates a network security group with the specified open ports
func (p *AzureProvider) CreateNSG(
	ctx context.Context,
	deployment *models.Deployment,
	location string,
	disp *display.Display,
) (*armnetwork.SecurityGroup, error) {
	l := logger.Get()
	l.Debugf("CreateNSG: %s", location)

	for _, machine := range deployment.Machines {
		if machine.Location == location {
			disp.UpdateStatus(&models.Status{
				ID:     machine.ID,
				Type:   "VM",
				Status: "Creating NSG for " + location,
			})
		}
	}

	createdNSG, err := p.Client.CreateNetworkSecurityGroup(
		ctx,
		deployment.ResourceGroupName,
		location,
		deployment.AllowedPorts,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &createdNSG, nil
}
