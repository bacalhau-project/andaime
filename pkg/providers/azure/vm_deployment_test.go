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
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
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
	mockClient.(*MockAzureClient).CreatePublicIPFunc = func(ctx context.Context, resourceGroupName, location, ipName string, tags map[string]*string) (armnetwork.PublicIPAddress, error) {
		return testdata.TestPublicIPAddress, nil
	}

	mockClient.(*MockAzureClient).CreateNetworkInterfaceFunc = func(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface, tags map[string]*string) (armnetwork.Interface, error) {
		return testdata.TestInterface, nil
	}

	mockClient.(*MockAzureClient).CreateVirtualMachineFunc = func(ctx context.Context, resourceGroupName, vmName string, parameters armcompute.VirtualMachine, tags map[string]*string) (armcompute.VirtualMachine, error) {
		return testdata.TestVirtualMachine, nil
	}

	mockClient.(*MockAzureClient).CreateNetworkSecurityGroupFunc = func(ctx context.Context, resourceGroupName, sgName string, location string, ports []int, tags map[string]*string) (armnetwork.SecurityGroup, error) {
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

	deployment := &models.Deployment{
		ProjectID:         "testProject",
		UniqueID:          "testUniqueID",
		AllowedPorts:      []int{22, 80, 443},
		ResourceGroupName: "testRG",
		Subnets: map[string][]*armnetwork.Subnet{
			"eastus": {&testdata.TestSubnet},
		},
		SSHPublicKeyPath:  testSSHPublicKeyPath,
		SSHPrivateKeyPath: testSSHPrivateKeyPath,
	}

	createdVM, err := DeployVM(ctx,
		mockClient,
		deployment,
		&models.Machine{
			Location:             "eastus",
			PublicIP:             &testdata.TestPublicIPAddress,
			NetworkSecurityGroup: &testdata.TestNSG,
			Interface:            &testdata.TestInterface,
		},
		&display.Display{},
	)
	if err != nil {
		t.Errorf("DeployVM failed: %v", err)
	}

	assert.Equal(t, *createdVM.Name, "testVM")
}

func TestCreatePublicIP(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockAzureClient()
	mockClient.(*MockAzureClient).CreatePublicIPFunc = func(ctx context.Context, resourceGroupName, location, ipName string, tags map[string]*string) (armnetwork.PublicIPAddress, error) {
		return testdata.TestPublicIPAddress, nil
	}

	deployment := &models.Deployment{
		ProjectID:         "testProject",
		UniqueID:          "testUniqueID",
		ResourceGroupName: "testRG",
		Locations: []string{
			"eastus",
		},
		Subnets: map[string][]*armnetwork.Subnet{
			"eastus": {
				{
					Name: to.Ptr("testSubnet"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr("10.0.0.0/24"),
					},
				},
			},
		},
	}

	machine := &models.Machine{
		Location: "eastus",
	}
	display := &display.Display{}

	publicIP, err := createPublicIP(
		ctx,
		mockClient,
		deployment,
		machine,
		display,
	)
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
	mockClient.(*MockAzureClient).CreateNetworkSecurityGroupFunc = func(ctx context.Context, resourceGroupName, sgName string, location string, ports []int, tags map[string]*string) (armnetwork.SecurityGroup, error) {
		return testdata.TestNSG, nil
	}

	deployment := &models.Deployment{}
	machine := &models.Machine{}
	display := &display.Display{}

	nsg, err := createNSG(
		ctx,
		mockClient,
		deployment,
		machine,
		display,
	)
	if err != nil {
		t.Errorf("createNSG failed: %v", err)
	}
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
	mockClient.(*MockAzureClient).CreateNetworkInterfaceFunc = func(ctx context.Context, resourceGroupName, nicName string, parameters armnetwork.Interface, tags map[string]*string) (armnetwork.Interface, error) {
		return testdata.TestInterface, nil
	}
	deployment := &models.Deployment{
		ProjectID:         "testProject",
		UniqueID:          "testUniqueID",
		ResourceGroupName: "testRG",
		Locations: []string{
			"eastus",
		},
	}
	deployment.Tags = GenerateTags(deployment.ProjectID, deployment.UniqueID)
	machine := &models.Machine{
		Location: "eastus",
	}
	display := &display.Display{}

	deployment.SetSubnet("eastus", &testdata.TestSubnet)

	machine.PublicIP = &testdata.TestPublicIPAddress
	machine.NetworkSecurityGroup = &testdata.TestNSG

	nic, err := createNIC(
		ctx,
		mockClient,
		deployment,
		machine,
		display,
	)
	if err != nil {
		t.Errorf("createNIC failed: %v", err)
	}
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
	mockClient.(*MockAzureClient).CreateVirtualMachineFunc = func(ctx context.Context,
		resourceGroupName, vmName string,
		parameters armcompute.VirtualMachine,
		tags map[string]*string) (armcompute.VirtualMachine, error) {
		return testdata.TestVirtualMachine, nil
	}
	projectID := "testProject"
	uniqueID := "testUniqueID"

	tags := GenerateTags(projectID, uniqueID)

	deployment := &models.Deployment{}
	deployment.Tags = tags
	deployment.ResourceGroupName = "testRG"
	deployment.UniqueID = "testUniqueID"
	deployment.Locations = []string{"eastus"}

	machine := &models.Machine{}
	machine.Interface = &testdata.TestInterface
	machine.Location = "eastus"
	machine.PublicIP = &testdata.TestPublicIPAddress
	machine.NetworkSecurityGroup = &testdata.TestNSG

	display := &display.Display{}
	createdVM, err := createVM(
		ctx,
		mockClient,
		deployment,
		machine,
		display,
	)
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
