package azure

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/sshutils"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

func TestDeployVM(t *testing.T) {
	ctx := context.Background()
	testSSHPublicKeyPath, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockClient := NewMockAzureClient()
	// Create a mock for each method that will be called
	mockClient.(*MockAzureClient).CreateVirtualNetworkFunc = func(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
		return testdata.TestVirtualNetwork, nil
	}

	mockClient.(*MockAzureClient).CreatePublicIPFunc = func(ctx context.Context, resourceGroupName, ipName string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddress, error) {
		return testdata.TestPublicIPAddress, nil
	}

	mockClient.(*MockAzureClient).CreateNetworkInterfaceFunc = func(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface) (armnetwork.Interface, error) {
		return testdata.TestInterface, nil
	}

	mockClient.(*MockAzureClient).CreateVirtualMachineFunc = func(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine) (armcompute.VirtualMachine, error) {
		return testdata.TestVirtualMachine, nil
	}

	mockClient.(*MockAzureClient).CreateNetworkSecurityGroupFunc = func(ctx context.Context, resourceGroupName, sgName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroup, error) {
		return testdata.TestNSG, nil
	}

	sshutils.SSHRetryAttempts = 1
	sshutils.SSHRetryDelay = 1

	mockSSHSession := sshutils.NewMockSSHSession()
	mockSSHSession.On("Close", mock.Anything).Return(nil)
	mockSSHClient, _ := sshutils.NewMockSSHClient(nil)
	mockSSHClient.On("NewSession", mock.Anything).Return(mockSSHSession, nil)
	mockSSHClient.On("Close", mock.Anything).Return(nil)

	sshutils.NewSSHClientFunc = func(sshClientConfig *ssh.ClientConfig, dialer sshutils.SSHDialer) sshutils.SSHClienter {
		return mockSSHClient
	}

	viper.Reset()
	viper.SetConfigFile("config.yaml")
	viper.ReadConfig(bytes.NewBufferString(testdata.TestAzureConfig))

	viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)

	createdVM, err := DeployVM(ctx,
		"testProject",
		"testUniqueID",
		mockClient,
		viper.GetViper(),
	)
	if err != nil {
		t.Errorf("DeployVM failed: %v", err)
	}

	assert.Equal(t, *createdVM.Name, "testVM")
}

func TestCreateVirtualNetwork(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	subnetName := "testSubnet"
	addressPrefix := "10.0.0.0/24"
	mockClient.(*MockAzureClient).CreateVirtualNetworkFunc = func(ctx context.Context, resourceGroupName, vnetName string, parameters armnetwork.VirtualNetwork) (armnetwork.VirtualNetwork, error) {
		tVN := testdata.TestVirtualNetwork
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
		return testdata.TestPublicIPAddress, nil
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
		return testdata.TestNSG, nil
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
		return testdata.TestInterface, nil
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
		return testdata.TestVirtualMachine, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"

	tags := generateTags(projectID, uniqueID)

	nicID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkInterfaces/testNIC"
	createdVM, err := createVM(ctx, projectID, uniqueID, mockClient, "testRG", "testVM", "eastus", nicID, 30, "Standard_DS1_v2", testdata.TestPublicSSHKeyMaterial, tags)
	if err != nil {
		t.Errorf("createVM failed: %v", err)
	}

	assert.Equal(t, *createdVM.Name, "testVM")
}

func TestWaitForSSH(t *testing.T) {
	// Set test mode
	os.Setenv("ANDAIME_TEST_MODE", "true")
	defer os.Unsetenv("ANDAIME_TEST_MODE")

	sshWaiter := sshutils.DefaultSSHWaiter{}
	mockSSHDialer := sshutils.NewMockSSHDialer()
	mockSSHSession := sshutils.NewMockSSHSession()
	mockSSHSession.On("Close", mock.Anything).Return(nil)

	mockSSHClient, _ := sshutils.NewMockSSHClient(mockSSHDialer)
	mockSSHClient.On("NewSession", mock.Anything).Return(mockSSHSession, nil)
	mockSSHClient.On("Close", mock.Anything).Return(nil)
	sshWaiter.Client = mockSSHClient

	oldSSHClientFunc := sshutils.NewSSHClientFunc
	sshutils.NewSSHClientFunc = func(sshClientConfig *ssh.ClientConfig, dialer sshutils.SSHDialer) sshutils.SSHClienter {
		return mockSSHClient
	}
	defer func() {
		sshutils.NewSSHClientFunc = oldSSHClientFunc
	}()

	// This test is a bit tricky because it involves networking.
	// We'll create a simple mock by overriding the global ssh.Dial function.
	sshutils.SSHKeyReader = sshutils.MockSSHKeyReader

	// This test should pass because the private key is valid
	err := sshWaiter.WaitForSSH(&sshutils.SSHConfig{
		Host:               "1.2.3.4",
		User:               "testuser",
		PrivateKeyMaterial: testdata.TestPrivateSSHKeyMaterial,
		SSHDialer:          mockSSHDialer,
	})
	if err != nil {
		t.Errorf("waitForSSH failed: %v", err)
	}

	// This test should fail because the private key is not valid
	err = sshWaiter.WaitForSSH(&sshutils.SSHConfig{
		Host:               "1.2.3.4",
		User:               "testuser",
		PrivateKeyMaterial: "mockprivatekey",
	})
	if err == nil {
		t.Error("waitForSSH should have failed with timeout")
	}
}
