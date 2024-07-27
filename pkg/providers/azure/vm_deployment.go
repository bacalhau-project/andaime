package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/utils"
)

var (
	basePriority = 100
)

func DeployVM(ctx context.Context,
	client AzureClient,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armcompute.VirtualMachine, error) {
	absPrivateKeyPath, err := utils.ExpandPath(deployment.SSHPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %v", err)
	}

	// Create Public IP
	publicIP, err := createPublicIP(
		ctx,
		client,
		deployment,
		machine,
		disp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP: %v", err)
	}

	machine.PublicIP = publicIP

	// Create Network Interface
	nic, err := createNIC(
		ctx,
		client,
		deployment,
		machine,
		disp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	machine.Interface = nic

	// Create Virtual Machine
	createdVM, err := createVM(
		ctx,
		client,
		deployment,
		machine,
		disp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual machine: %v", err)
	}

	// Wait for SSH to be available
	publicIPAddress := ""
	if publicIP.Properties != nil && publicIP.Properties.IPAddress != nil {
		publicIPAddress = *publicIP.Properties.IPAddress
	}

	sshDialer := &sshutils.MockSSHDialer{} //nolint:gomnd
	sshDialer.On("Dial", "tcp", fmt.Sprintf("%s:22", publicIPAddress)).Return(nil, nil)
	sshConfig, err := sshutils.NewSSHConfig(
		publicIPAddress,
		22, //nolint:gomnd
		"azureuser",
		sshDialer,
		absPrivateKeyPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH config: %v", err)
	}
	err = sshutils.SSHWaiterFunc(sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SSH connection: %v", err)
	}

	return createdVM, nil
}

// createPublicIP creates a public IP address
func createPublicIP(
	ctx context.Context,
	client AzureClient,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.PublicIPAddress, error) {
	createdIP, err := client.CreatePublicIP(
		ctx,
		deployment.ResourceGroupName,
		deployment.UniqueID,
		machine.Location,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP: %v", err)
	}

	return &createdIP, nil
}

// createNIC creates a network interface with both public and private IP
func createNIC(
	ctx context.Context,
	client AzureClient,
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

	nic := armnetwork.Interface{
		Location: to.Ptr(machine.Location),
		Tags:     deployment.Tags,
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet:                    deployment.Subnets[machine.Location][0],
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						PublicIPAddress:           machine.PublicIP,
					},
				},
			},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: machine.NetworkSecurityGroup.ID,
			},
		},
	}

	createdNIC, err := client.CreateNetworkInterface(
		ctx,
		deployment.ResourceGroupName,
		deployment.UniqueID+"-nic",
		nic,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	return &createdNIC, nil
}

// createVM creates the virtual machine
func createVM(
	ctx context.Context,
	client AzureClient,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armcompute.VirtualMachine, error) {
	vm := armcompute.VirtualMachine{
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

	createdVM, err := client.CreateVirtualMachine(
		ctx,
		deployment.ResourceGroupName,
		deployment.UniqueID,
		vm,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual machine: %v", err)
	}

	return &createdVM, nil
}

// createNSG creates a network security group with the specified open ports
func createNSG(
	ctx context.Context,
	client AzureClient,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.SecurityGroup, error) {
	createdNSG, err := client.CreateNetworkSecurityGroup(
		ctx,
		deployment.ResourceGroupName,
		deployment.UniqueID+"-nsg",
		machine.Location,
		deployment.AllowedPorts,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &createdNSG, nil
}
