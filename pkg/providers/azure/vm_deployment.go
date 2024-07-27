package azure

import (
	"context"
	"fmt"
	"time"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

var (
	basePriority = 100
)

// createPublicIP creates a public IP address with retry logic
func (p *AzureProvider) CreatePublicIP(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.PublicIPAddress, error) {
	l := logger.Get()
	maxRetries := 5
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		l.Debugf("CreatePublicIP: Attempt %d of %d for machine %s", attempt+1, maxRetries, machine.ID)

		createdIP, err := p.Client.CreatePublicIP(
			ctx,
			deployment.ResourceGroupName,
			machine.Location,
			machine.ID,
			deployment.Tags,
		)

		if err == nil {
			l.Infof("CreatePublicIP: Successfully created public IP for machine %s on attempt %d", machine.ID, attempt+1)
			return &createdIP, nil
		}

		lastErr = err
		if strings.Contains(err.Error(), "Canceled") {
			l.Warnf("CreatePublicIP: Operation was canceled, retrying for machine %s: %v", machine.ID, err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		l.Errorf("CreatePublicIP: Failed to create public IP for machine %s: %v", machine.ID, err)
		return nil, fmt.Errorf("failed to create public IP after %d attempts: %v", maxRetries, err)
	}

	return nil, fmt.Errorf("failed to create public IP after %d attempts: %v", maxRetries, lastErr)
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

	defaultMachineType := viper.GetString("azure.default_machine_type")
	if defaultMachineType == "" {
		defaultMachineType = "Standard_DS1_v2"
	}

	vmSize := machine.VMSize
	if vmSize == "" {
		vmSize = defaultMachineType
	}

	params := armcompute.VirtualMachine{
		Location: to.Ptr(machine.Location),
		Tags:     deployment.Tags,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(vmSize)),
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
					DiskSizeGB: to.Ptr(getDiskSizeGB(machine.DiskSizeGB)),
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

	nsgName := fmt.Sprintf("%s-%s-nsg", deployment.ResourceGroupName, location)
	createdNSG, err := p.Client.CreateNetworkSecurityGroup(
		ctx,
		deployment.ResourceGroupName,
		location,
		nsgName,
		deployment.AllowedPorts,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &createdNSG, nil
}

// getDiskSizeGB returns the disk size in GB, using a default value from config if not set
func getDiskSizeGB(diskSize int32) int32 {
	if diskSize <= 0 {
		defaultDiskSize := viper.GetInt32("azure.disk_size_gb")
		if defaultDiskSize <= 0 {
			return 30 // Fallback default if not set in config
		}
		return defaultDiskSize
	}
	return diskSize
}
