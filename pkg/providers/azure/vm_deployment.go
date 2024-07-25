package azure

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/utils"
	"github.com/spf13/viper"
)

var (
	basePriority = 100
)

func DeployVM(ctx context.Context,
	projectID, uniqueID string,
	client AzureClient,
	config *viper.Viper,
	resourceGroupName string,
	location string,
	vmSize string,
	subnet *armnetwork.Subnet,
) (*armcompute.VirtualMachine, error) {
	// Read configuration values
	diskSizeGB := config.GetInt32("azure.disk_size_gb")
	ports := config.GetIntSlice("azure.allowed_ports")
	publicKeyPath := config.GetString("general.ssh_public_key_path")
	privateKeyPath := config.GetString("general.ssh_private_key_path")

	tags := generateTags(uniqueID, projectID)
	vmName := utils.GenerateUniqueName(projectID, uniqueID)

	absKeyPath, err := utils.ExpandPath(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %v", err)
	}

	SSHPublicKeyMaterial, err := os.ReadFile(absKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH public key: %v", err)
	}

	absPrivateKeyPath, err := utils.ExpandPath(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %v", err)
	}

	_, err = os.ReadFile(absPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH private key: %v", err)
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
		return nil, fmt.Errorf("failed to create public IP: %v", err)
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
		return nil, fmt.Errorf("failed to create network security group: %v", err)
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
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	// Create Virtual Machine
	createdVM, err := createVM(
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
		vmSize,
		tags,
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
		config.GetString("azure.admin_username"),
		sshDialer,
		privateKeyPath,
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

	// In the createVirtualNetwork function, before creating the subnet:
	log := logger.Get()
	log.Debugf("Creating subnet with the following details:")
	log.Debugf("  Name: %s", *subnet.Name)
	log.Debugf("  Address Prefix: %s", *subnet.Properties.AddressPrefix)

	createdVNet, err := client.CreateVirtualNetwork(ctx, resourceGroupName, vnetName, location, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual network: %v", err)
	}

	log.Debugf("Virtual network created successfully:")
	log.Debugf("  ID: %s", *createdVNet.ID)
	log.Debugf("  Name: %s", *createdVNet.Name)
	log.Debugf("  Location: %s", *createdVNet.Location)
	if createdVNet.Properties != nil && createdVNet.Properties.ProvisioningState != nil {
		log.Debugf("  Provisioning State: %s", *createdVNet.Properties.ProvisioningState)
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

	EnsureTags(tags, projectID, uniqueID)

	createdIP, err := client.CreatePublicIP(ctx, resourceGroupName, ipName, publicIP, tags)
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
	EnsureTags(tags, projectID, uniqueID)

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

	createdNIC, err := client.CreateNetworkInterface(ctx, resourceGroupName, nicName, nic, tags)
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
	vmSize string,
	tags map[string]*string,
) (*armcompute.VirtualMachine, error) {
	EnsureTags(tags, projectID, uniqueID)

	vm := armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(vmSize)),
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

	createdVM, err := client.CreateVirtualMachine(ctx, resourceGroupName, vmName, vm, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual machine: %v", err)
	}

	return &createdVM, nil
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

	EnsureTags(tags, projectID, uniqueID)

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

	createdNSG, err := client.CreateNetworkSecurityGroup(ctx, resourceGroupName, nsgName, nsg, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &createdNSG, nil
}
