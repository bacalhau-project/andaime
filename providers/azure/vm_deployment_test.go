package azure

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/utils"
	"golang.org/x/crypto/ssh"
)

func TestDeployVM(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	// Create a mock for each method that will be called
	mockClient.(*MockAzureClient).CreateVirtualNetworkFunc = func(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
		return testVirtualNetwork, nil
	}

	mockClient.(*MockAzureClient).CreatePublicIPFunc = func(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
		return testPublicIPAddress, nil
	}

	mockClient.(*MockAzureClient).CreateNetworkInterfaceFunc = func(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
		return testInterface, nil
	}

	mockClient.(*MockAzureClient).CreateVirtualMachineFunc = func(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
		return testVirtualMachine, nil
	}

	mockClient.(*MockAzureClient).CreateNetworkSecurityGroupFunc = func(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
		return testNSG, nil
	}

	utils.SSHKeyReader = utils.MockSSHKeyReader
	sshDial = func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) { return &ssh.Client{}, nil }

	err := DeployVM(ctx, "testProject",
		"testUniqueID",
		mockClient,
		"testRG",
		"eastus",
		"testVM",
		"DS1_v2",
		30,
		[]int{22, 80, 443},
		"/null/path",
	)
	if err != nil {
		t.Errorf("DeployVM failed: %v", err)
	}

}

func TestCreateVirtualNetwork(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	subnetName := "testSubnet"
	addressPrefix := "10.0.0.0/24"
	mockClient.(*MockAzureClient).CreateVirtualNetworkFunc = func(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
		tVN := testVirtualNetwork
		tVN.Name = to.Ptr(subnetName)
		tVN.Properties.Subnets[0].Properties.AddressPrefix = to.Ptr(addressPrefix)
		return tVN, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"
	tags := generateTags(projectID, uniqueID)

	subnet, err := createVirtualNetwork(ctx, projectID, uniqueID, mockClient, "testRG", "testVNet", "testSubnet", "eastus", tags)
	if err != nil {
		t.Errorf("createVirtualNetwork failed: %v", err)
	}

	if subnet == nil {
		t.Error("createVirtualNetwork returned nil subnet")
	}

	if subnet != nil && *subnet.Name != subnetName {
		t.Errorf("Expected subnet name '%s', got '%s'", subnetName, *subnet.Name)
	}

	if subnet != nil && *subnet.Properties.AddressPrefix != addressPrefix {
		t.Errorf("Expected subnet address prefix %s, got '%s'", subnetName, *subnet.Properties.AddressPrefix)
	}
}

func TestCreatePublicIP(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	mockClient.(*MockAzureClient).CreatePublicIPFunc = func(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
		return testPublicIPAddress, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"
	tags := generateTags(projectID, uniqueID)

	publicIP, err := createPublicIP(ctx, projectID, uniqueID, mockClient, "testRG", "testIP", "eastus", tags)
	if err != nil {
		t.Errorf("createPublicIP failed: %v", err)
	}

	if publicIP == nil {
		t.Error("createPublicIP returned nil public IP")
	}
}

func TestCreateNSG(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	mockClient.(*MockAzureClient).CreateNetworkSecurityGroupFunc = func(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
		return testNSG, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"
	tags := generateTags(projectID, uniqueID)

	nsg, err := createNSG(ctx, projectID, uniqueID, mockClient, "testRG", "testNSG", "eastus", []int{22, 80, 443}, tags)
	if err != nil {
		t.Errorf("createNSG failed: %v", err)
	}

	if nsg == nil {
		t.Error("createNSG returned nil NSG")
	}
}

func TestCreateNIC(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	mockClient.(*MockAzureClient).CreateNetworkInterfaceFunc = func(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
		return testInterface, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"
	tags := generateTags(projectID, uniqueID)

	subnet := &armnetwork.Subnet{
		Name: to.Ptr("testSubnet"),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.0.0.0/24"),
		},
	}

	publicIP := &armnetwork.PublicIPAddress{
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			IPAddress: to.Ptr("1.2.3.4"),
		},
	}

	nsgID := to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkSecurityGroups/testNSG")

	nic, err := createNIC(ctx, projectID, uniqueID, mockClient, "testRG", "testNIC", "eastus", subnet, publicIP, nsgID, tags)
	if err != nil {
		t.Errorf("createNIC failed: %v", err)
	}

	if nic == nil {
		t.Error("createNIC returned nil NIC")
	}

}

func TestCreateVM(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	mockClient.(*MockAzureClient).CreateVirtualMachineFunc = func(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
		return testVirtualMachine, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"

	tags := generateTags(projectID, uniqueID)

	nicID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkInterfaces/testNIC"
	err := createVM(ctx, projectID, uniqueID, mockClient, "testRG", "testVM", "eastus", nicID, 30, utils.TestPublicSSHKey, tags)
	if err != nil {
		t.Errorf("createVM failed: %v", err)
	}
}

func TestWaitForSSH(t *testing.T) {
	// Set test mode
	os.Setenv("ANDAIME_TEST_MODE", "true")
	defer os.Unsetenv("ANDAIME_TEST_MODE")

	// This test is a bit tricky because it involves networking.
	// We'll create a simple mock by overriding the global ssh.Dial function.
	originalSSHDial := sshDial
	defer func() { sshDial = originalSSHDial }()

	sshDial = func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		return &ssh.Client{}, nil
	}

	utils.SSHKeyReader = utils.MockSSHKeyReader

	err := waitForSSH("1.2.3.4", "testuser", []byte(utils.TestPrivateSSHKey))
	if err != nil {
		t.Errorf("waitForSSH failed: %v", err)
	}

	// Test timeout scenario
	sshDial = func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		return nil, fmt.Errorf("connection refused")
	}

	err = waitForSSH("1.2.3.4", "testuser", []byte("mockprivatekey"))
	if err == nil {
		t.Error("waitForSSH should have failed with timeout")
	}
}

var testVirtualNetwork = armnetwork.VirtualNetwork{
	Name: to.Ptr("testVNet"),
	Properties: &armnetwork.VirtualNetworkPropertiesFormat{
		AddressSpace: &armnetwork.AddressSpace{
			AddressPrefixes: []*string{to.Ptr("10.0.0.0/16")},
		},
		Subnets: []*armnetwork.Subnet{
			{ // Subnet
				Name: to.Ptr("testSubnet"),
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix: to.Ptr("10.0.0.0/24"),
				},
			},
		},
	},
}

var testPublicIPAddress = armnetwork.PublicIPAddress{
	Name: to.Ptr("testIP"),
	Properties: &armnetwork.PublicIPAddressPropertiesFormat{
		IPAddress: to.Ptr("256.256.256.256"),
	},
}

var testInterface = armnetwork.Interface{
	ID:   to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkInterfaces/testNIC"),
	Name: to.Ptr("testNIC"),
	Properties: &armnetwork.InterfacePropertiesFormat{
		IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
			{
				Name:       to.Ptr("testIPConfig"),
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{},
			},
		},
	},
}

var testVirtualMachine = armcompute.VirtualMachine{
	Name: to.Ptr("testVM"),
	Properties: &armcompute.VirtualMachineProperties{
		StorageProfile: &armcompute.StorageProfile{
			ImageReference: &armcompute.ImageReference{
				Publisher: to.Ptr("Canonical"),
				Offer:     to.Ptr("UbuntuServer"),
				Version:   to.Ptr("latest"),
			},
		},
		OSProfile: &armcompute.OSProfile{
			ComputerName:  to.Ptr("testVM"),
			AdminUsername: to.Ptr("testuser"),
			AdminPassword: to.Ptr("testpassword"),
			LinuxConfiguration: &armcompute.LinuxConfiguration{
				SSH: &armcompute.SSHConfiguration{
					PublicKeys: []*armcompute.SSHPublicKey{
						{Path: to.Ptr("/home/testuser/.ssh/authorized_keys"),
							KeyData: to.Ptr(utils.TestPublicSSHKey),
						},
					},
				},
			},
		},
		HardwareProfile: &armcompute.HardwareProfile{
			VMSize: &armcompute.PossibleVirtualMachineSizeTypesValues()[0],
		},
	},
}

var testNSG = armnetwork.SecurityGroup{
	Name: to.Ptr("testNSG"),
	Properties: &armnetwork.SecurityGroupPropertiesFormat{
		SecurityRules: []*armnetwork.SecurityRule{
			{
				Name: to.Ptr("testRule"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
					Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					DestinationAddressPrefix: to.Ptr("*"),
					DestinationPortRange:     to.Ptr("22"),
					Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
					SourceAddressPrefix:      to.Ptr("*"),
					SourcePortRange:          to.Ptr("*"),
				},
			},
		},
	},
}
