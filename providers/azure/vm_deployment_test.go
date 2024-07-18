package azure

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"golang.org/x/crypto/ssh"
)

const TestPublicSSHKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC4VNbBAdUjsEGtthi6f804ftcSer2BUHJ4n4I2olBOB dummy@example.com"

var TestPrivateSSHKey = []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAuFTWwQHVI7BBrbYYun/NOH7XEnq9gVByeJ+CNqJQTgQAAAJg1FTcNNRU3
DQAAAAtzc2gtZWQyNTUxOQAAACAuFTWwQHVI7BBrbYYun/NOH7XEnq9gVByeJ+CNqJQTgQ
AAAEAiSKPZOlligMHdH5BZdobDWhuyMkR+mR/s16zklfhFii4VNbBAdUjsEGtthi6f804f
tcSer2BUHJ4n4I2olBOBAAAAEWR1bW15QGV4YW1wbGUuY29tAQIDBA==
-----END OPENSSH PRIVATE KEY-----`)

func TestDeployVM(t *testing.T) {
	ctx := context.Background()
	clients := NewMockClientInterfaces()

	err := DeployVM(ctx, "testProject",
		"testUniqueID",
		clients,
		"testRG",
		"eastus",
		"testVM",
		"DS1_v2",
		30,
		[]int{22, 80, 443},
		TestPublicSSHKey,
	)
	if err != nil {
		t.Errorf("DeployVM failed: %v", err)
	}
}

func TestCreateVirtualNetwork(t *testing.T) {
	ctx := context.Background()
	client := &MockVirtualNetworksClient{}
	tags := generateTags("testProject", "testUniqueID")

	subnet, err := createVirtualNetwork(ctx, "testProject", "testUUID", client, "testRG", "testVNet", "testSubnet", "eastus", tags)
	if err != nil {
		t.Errorf("createVirtualNetwork failed: %v", err)
	}

	if subnet == nil {
		t.Error("createVirtualNetwork returned nil subnet")
	}

	if subnet != nil && *subnet.Name != "test-subnet" {
		t.Errorf("Expected subnet name 'test-subnet', got '%s'", *subnet.Name)
	}

	if subnet != nil && *subnet.Properties.AddressPrefix != "10.0.0.0/24" {
		t.Errorf("Expected subnet address prefix '10.0.0.0/24', got '%s'", *subnet.Properties.AddressPrefix)
	}
}

func TestCreatePublicIP(t *testing.T) {
	ctx := context.Background()
	client := &MockPublicIPAddressesClient{}
	tags := generateTags("testProject", "testUniqueID")

	publicIP, err := createPublicIP(ctx, "testProject", "testUUID", client, "testRG", "testIP", "eastus", tags)
	if err != nil {
		t.Errorf("createPublicIP failed: %v", err)
	}

	if publicIP == nil {
		t.Error("createPublicIP returned nil public IP")
	}

	if publicIP != nil && publicIP.Properties != nil && *publicIP.Properties.IPAddress != "1.2.3.4" {
		t.Errorf("Expected IP address '1.2.3.4', got '%s'", *publicIP.Properties.IPAddress)
	}
}

func TestCreateNSG(t *testing.T) {
	ctx := context.Background()
	client := &MockSecurityGroupsClient{}

	tags := generateTags("testProject", "testUniqueID")

	nsg, err := createNSG(ctx, "testProject", "testUniqueID", client, "testRG", "testNSG", "eastus", []int{22, 80, 443}, tags)
	if err != nil {
		t.Errorf("createNSG failed: %v", err)
	}

	if nsg == nil {
		t.Error("createNSG returned nil NSG")
	}
}

func TestCreateNIC(t *testing.T) {
	ctx := context.Background()
	client := &MockNetworkInterfacesClient{}
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

	nic, err := createNIC(ctx, projectID, uniqueID, client, "testRG", "testNIC", "eastus", subnet, publicIP, nsgID, tags)
	if err != nil {
		t.Errorf("createNIC failed: %v", err)
	}

	if nic == nil {
		t.Error("createNIC returned nil NIC")
	}
}

func TestCreateVM(t *testing.T) {
	ctx := context.Background()
	client := &MockVirtualMachinesClient{}

	tags := generateTags("testProject", "testUniqueID")

	nicID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkInterfaces/testNIC"
	err := createVM(ctx, "testProject", "testUniqueID", client, "testRG", "testVM", "eastus", nicID, 30, TestPublicSSHKey, tags)
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

	err := waitForSSH("1.2.3.4", "testuser", []byte("mockprivatekey"))
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

// TODO: Implement mock methods for other client interfaces as needed...
