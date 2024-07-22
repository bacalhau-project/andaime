package azure

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

var (
	basePriority = 100
)

// DeployVM creates an Azure VM with the specified configuration
func DeployVM(ctx context.Context,
	projectID, uniqueID string,
	client AzureClient,
	resourceGroupName, location, vmName, vmSize string,
	diskSizeGB int32,
	ports []int,
	publicKeyPath, privateKeyPath string,
) error {
	tags := generateTags(uniqueID, projectID)
	SSHPublicKeyMaterial, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH public key: %v", err)
	}

	_, err = os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH private key: %v", err)
	}

	// Create Virtual Network and Subnet
	subnet, err := createVirtualNetwork(
		ctx,
		projectID,
		uniqueID,
		client,
		resourceGroupName,
		vmName+"-vnet",
		vmName+"-subnet",
		location,
		tags,
	)
	if err != nil {
		return fmt.Errorf("failed to create virtual network: %v", err)
	}

	// Create Public IP
	publicIP, err := createPublicIP(
		ctx,
		projectID,
		uniqueID,
		client,
		resourceGroupName,
		vmName+"-ip",
		location,
		tags,
	)
	if err != nil {
		return fmt.Errorf("failed to create public IP: %v", err)
	}

	// Create Network Security Group
	nsg, err := createNSG(
		ctx,
		projectID,
		uniqueID,
		client,
		resourceGroupName,
		vmName+"-nsg",
		location,
		ports,
		tags,
	)
	if err != nil {
		return fmt.Errorf("failed to create network security group: %v", err)
	}

	// Create Network Interface
	nic, err := createNIC(
		ctx,
		projectID,
		uniqueID,
		client,
		resourceGroupName,
		vmName+"-nic",
		location,
		subnet,
		publicIP,
		nsg.ID,
		tags,
	)
	if err != nil {
		return fmt.Errorf("failed to create network interface: %v", err)
	}

	// Create Virtual Machine
	err = createVM(
		ctx,
		projectID,
		uniqueID,
		client,
		resourceGroupName,
		vmName,
		location,
		*nic.ID,
		diskSizeGB,
		string(SSHPublicKeyMaterial),
		tags,
	)
	if err != nil {
		return fmt.Errorf("failed to create virtual machine: %v", err)
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
		privateKeyPath,
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %v", err)
	}
	err = sshutils.SSHWaiterFunc(sshConfig)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}

	return nil
}

// createVirtualNetwork creates a virtual network and subnet
func createVirtualNetwork(ctx context.Context,
	projectID, uniqueID string,
	client AzureClient,
	resourceGroupName, vnetName, subnetName, location string,
	tags map[string]*string) (*armnetwork.Subnet, error) {
	subnet := armnetwork.Subnet{
		Name: to.Ptr(subnetName),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.0.0.0/24"),
		},
	}

	ensureTags(tags, projectID, uniqueID)

	vnet := armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.0.0.0/16")},
			},
			Subnets: []*armnetwork.Subnet{&subnet},
		},
	}

	createdVNet, err := client.CreateVirtualNetwork(ctx, resourceGroupName, vnetName, vnet)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual network: %v", err)
	}

	if createdVNet.Properties != nil && createdVNet.Properties.Subnets != nil && len(createdVNet.Properties.Subnets) > 0 {
		return createdVNet.Properties.Subnets[0], nil
	}

	return nil, fmt.Errorf("subnet not found in the created virtual network")
}

// createPublicIP creates a public IP address
func createPublicIP(
	ctx context.Context,
	projectID string,
	uniqueID string,
	client AzureClient,
	resourceGroupName, ipName, location string,
	tags map[string]*string,
) (*armnetwork.PublicIPAddress, error) {
	publicIP := armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}

	ensureTags(tags, projectID, uniqueID)

	createdIP, err := client.CreatePublicIP(ctx, resourceGroupName, ipName, publicIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP: %v", err)
	}

	return &createdIP, nil
}

// createNIC creates a network interface with both public and private IP
func createNIC(
	ctx context.Context,
	projectID, uniqueID string,
	client AzureClient,
	resourceGroupName, nicName, location string,
	subnet *armnetwork.Subnet,
	publicIP *armnetwork.PublicIPAddress,
	nsgID *string,
	tags map[string]*string,
) (*armnetwork.Interface, error) {
	ensureTags(tags, projectID, uniqueID)

	nic := armnetwork.Interface{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet:                    subnet,
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						PublicIPAddress:           publicIP,
					},
				},
			},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsgID,
			},
		},
	}

	createdNIC, err := client.CreateNetworkInterface(ctx, resourceGroupName, nicName, nic)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	return &createdNIC, nil
}

// createVM creates the virtual machine
func createVM(
	ctx context.Context,
	projectID, uniqueID string,
	client AzureClient,
	resourceGroupName, vmName, location, nicID string,
	diskSizeGB int32,
	sshPublicKeyMaterial string,
	tags map[string]*string,
) error {
	ensureTags(tags, projectID, uniqueID)

	vm := armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_DS1_v2")),
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(vmName),
				AdminUsername: to.Ptr("azureuser"),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr("/home/azureuser/.ssh/authorized_keys"),
								KeyData: to.Ptr(sshPublicKeyMaterial),
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
					DiskSizeGB: to.Ptr(diskSizeGB),
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(nicID),
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							Primary: to.Ptr(true),
						},
					},
				},
			},
		},
	}

	_, err := client.CreateVirtualMachine(ctx, resourceGroupName, vmName, vm)
	if err != nil {
		return fmt.Errorf("failed to create virtual machine: %v", err)
	}

	return nil
}

// createNSG creates a network security group with the specified open ports
func createNSG(
	ctx context.Context,
	projectID, uniqueID string,
	client AzureClient,
	resourceGroupName, nsgName, location string,
	ports []int,
	tags map[string]*string,
) (*armnetwork.SecurityGroup, error) {
	var securityRules []*armnetwork.SecurityRule

	ensureTags(tags, projectID, uniqueID)

	for i, port := range ports {
		ruleName := fmt.Sprintf("Allow-%d", port)
		securityRules = append(securityRules, &armnetwork.SecurityRule{
			Name: to.Ptr(ruleName),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
				SourceAddressPrefix:      to.Ptr("*"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("*"),
				DestinationPortRange:     to.Ptr(fmt.Sprintf("%d", port)),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
				Priority:                 to.Ptr(int32(basePriority + i)),
			},
		})
	}

	nsg := armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: securityRules,
		},
	}

	createdNSG, err := client.CreateNetworkSecurityGroup(ctx, resourceGroupName, nsgName, nsg)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &createdNSG, nil
}
