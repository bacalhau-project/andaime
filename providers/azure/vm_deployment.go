package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"golang.org/x/crypto/ssh"
)

var sshDial = ssh.Dial

var (
	sshTimeOut       = 30 * time.Second
	sshRetryAttempts = 30
	sshRetryDelay    = 10 * time.Second
	pollingDelay     = 1 * time.Second
	basePriority     = 100
)

// DeployVM creates an Azure VM with the specified configuration
func DeployVM(ctx context.Context,
	projectID, uniqueID string,
	clients *ClientInterfaces,
	resourceGroupName, location, vmName, vmSize string,
	diskSizeGB int32,
	ports []int,
	sshPublicKey string,
) error {
	// TODO: Implement resource tracking system

	tags := generateTags(uniqueID, projectID)

	// Create Virtual Network and Subnet
	subnet, err := createVirtualNetwork(
		ctx,
		projectID,
		uniqueID,
		clients.VirtualNetworksClient,
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
		clients.PublicIPAddressesClient,
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
		clients.SecurityGroupsClient,
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
		clients.NetworkInterfacesClient,
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
		clients.VirtualMachinesClient,
		resourceGroupName,
		vmName,
		location,
		*nic.ID,
		diskSizeGB,
		sshPublicKey,
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
	err = waitForSSH(publicIPAddress, "azureuser", []byte(sshPublicKey))
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}

	return nil
}

// waitForSSH attempts to establish an SSH connection to the VM
func waitForSSH(publicIP string, username string, privateKey []byte) error {
	fmt.Println("Entering waitForSSH")
	fmt.Printf(
		"publicIP: %s, username: %s, privateKey length: %d\n",
		publicIP,
		username,
		len(privateKey),
	)

	if sshDial == nil {
		return fmt.Errorf("sshDial function is nil")
	}
	fmt.Println("sshDial function is not nil")

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}
	fmt.Println("Private key parsed successfully")

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         sshTimeOut,
	}
	fmt.Println("SSH client config created")

	for i := 0; i < sshRetryAttempts; i++ {
		fmt.Printf("Attempt %d to connect via SSH\n", i+1)
		client, err := sshDial("tcp", fmt.Sprintf("%s:22", publicIP), config)
		if err != nil {
			fmt.Printf("SSH dial error: %v\n", err)
			time.Sleep(sshRetryDelay)
			continue
		}

		if client == nil {
			fmt.Println("SSH client is nil despite no error")
			time.Sleep(sshRetryDelay)
			continue
		}

		fmt.Println("SSH connection established")
		if client.Conn != nil {
			client.Close()
		}
		return nil
	}

	return fmt.Errorf("failed to establish SSH connection after multiple attempts")
}

// createVirtualNetwork creates a virtual network and subnet
func createVirtualNetwork(ctx context.Context,
	projectID, uniqueID string,
	client VirtualNetworksClientAPI,
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

	pollerResp, err := client.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, vnet, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start creating virtual network: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual network: %v", err)
	}

	// Check if resp is valid
	if resp.VirtualNetwork.Properties == nil || resp.VirtualNetwork.Properties.Subnets == nil {
		return nil, fmt.Errorf("failed to create virtual network: invalid response")
	}

	getResp, err := client.Get(ctx, resourceGroupName, vnetName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual network: %v", err)
	}

	if getResp.Properties != nil && getResp.Properties.Subnets != nil &&
		len(getResp.Properties.Subnets) > 0 {
		return getResp.Properties.Subnets[0], nil
	}

	return nil, fmt.Errorf("subnet not found in the created virtual network")
}

// createPublicIP creates a public IP address
func createPublicIP(
	ctx context.Context,
	projectID, uniqueID string,
	client PublicIPAddressesClientAPI,
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

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroupName, ipName, publicIP, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start creating public IP: %v", err)
	}

	resp, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: pollingDelay})
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP: %v", err)
	}

	return &resp.PublicIPAddress, nil
}

// createNIC creates a network interface with both public and private IP
func createNIC(
	ctx context.Context,
	projectID, uniqueID string,
	client NetworkInterfacesClientAPI,
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

	pollerResp, err := client.BeginCreateOrUpdate(ctx, resourceGroupName, nicName, nic, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start creating network interface: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(
		ctx,
		&runtime.PollUntilDoneOptions{Frequency: pollingDelay},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	return &resp.Interface, nil
}

// createVM creates the virtual machine
func createVM(
	ctx context.Context,
	projectID, uniqueID string,
	client VirtualMachinesClientAPI,
	resourceGroupName, vmName, location, nicID string,
	diskSizeGB int32,
	sshPublicKey string,
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
								KeyData: to.Ptr(sshPublicKey),
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

	pollerResp, err := client.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, vm, nil)
	if err != nil {
		return fmt.Errorf("failed to start creating virtual machine: %v", err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual machine: %v", err)
	}

	return nil
}

// SecurityGroupsClientAPI defines the methods we need from SecurityGroupsClient
type SecurityGroupsClientAPI interface {
	BeginCreateOrUpdate(ctx context.Context,
		resourceGroupName string,
		networkSecurityGroupName string,
		parameters armnetwork.SecurityGroup,
		options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse], error) //nolint:lll
}

// createNSG creates a network security group with the specified open ports
func createNSG(
	ctx context.Context,
	projectID, uniqueID string,
	client SecurityGroupsClientAPI,
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

	pollerResp, err := client.BeginCreateOrUpdate(ctx, resourceGroupName, nsgName, nsg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start creating network security group: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(
		ctx,
		&runtime.PollUntilDoneOptions{Frequency: pollingDelay},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &resp.SecurityGroup, nil
}
