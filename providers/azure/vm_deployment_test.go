package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"golang.org/x/crypto/ssh"
)

const testPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFC/0bkfIjq9KR13/h1l4y+6+nr0LRFsrQKw3eu9ys8B dummy@example.com"

var testPrivateKey = []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBQv9G5HyI6vSkdd/4dZeMvuvp69C0RbK0CsN3rvcrPAQAAAJjp2+Uq6dvl
KgAAAAtzc2gtZWQyNTUxOQAAACBQv9G5HyI6vSkdd/4dZeMvuvp69C0RbK0CsN3rvcrPAQ
AAAEBb5gmhdxnI3fQUNApq//b269zU95FIPUy2/dATIaTsvVC/0bkfIjq9KR13/h1l4y+6
+nr0LRFsrQKw3eu9ys8BAAAAEWR1bW15QGV4YW1wbGUuY29tAQIDBA==
-----END OPENSSH PRIVATE KEY-----
`)

func TestDeployVM(t *testing.T) {
	ctx := context.Background()
	clients := getClientInterfaces()
	// Store the original sshDial function
	originalSSHDial := sshDial
	defer func() { sshDial = originalSSHDial }()

	// Create a mock sshDial function
	sshDial = func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		fmt.Println("Mock sshDial called")
		return &ssh.Client{}, nil
	}

	err := DeployVM(ctx,
		clients,
		"testRG",
		"testVM",
		"eastus",
		30,
		[]int{22, 80, 443},
		string(testPrivateKey),
		"uuid",
		"testProject")
	if err != nil {
		t.Errorf("DeployVM failed: %v", err)
	}
}

func TestCreateVirtualNetwork(t *testing.T) {
	ctx := context.Background()
	client := &MockVirtualNetworksClient{}

	// Create fake tags
	tags := generateTags("uuid", "testProject")

	subnet, err := createVirtualNetwork(
		ctx,
		client,
		"testRG",
		"testVNet",
		"testSubnet",
		"eastus",
		tags,
	)
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
		t.Errorf(
			"Expected subnet address prefix '10.0.0.0/24', got '%s'",
			*subnet.Properties.AddressPrefix,
		)
	}
}

func TestCreatePublicIP(t *testing.T) {
	ctx := context.Background()
	testIPAddress := "256.256.256.256"
	client := &MockPublicIPAddressesClient{IPAddress: testIPAddress}
	tags := generateTags("uuid", "testProject")
	publicIP, err := createPublicIP(ctx, client, "testRG", "testIP", "eastus", tags)
	if err != nil {
		t.Errorf("createPublicIP failed: %v", err)
	}

	if publicIP == nil {
		t.Error("createPublicIP returned nil public IP")
	}

	if publicIP != nil && publicIP.Properties != nil &&
		*publicIP.Properties.IPAddress != testIPAddress {
		t.Errorf(
			"Expected IP address '%s', got '%s'",
			testIPAddress,
			*publicIP.Properties.IPAddress,
		)
	}
}

func TestCreateNSG(t *testing.T) {
	ctx := context.Background()
	client := &MockSecurityGroupsClient{}
	tags := generateTags("uuid", "testProject")

	nsg, err := createNSG(
		ctx,
		client,
		"testRG",
		"testNSG",
		"eastus",
		[]int{22, 80, 443},
		tags,
	)
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
	tags := generateTags("uuid", "testProject")

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

	nsgID := to.Ptr(
		"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkSecurityGroups/testNSG",
	)

	nic, err := createNIC(
		ctx,
		client,
		"testRG",
		"testNIC",
		"eastus",
		subnet,
		publicIP,
		nsgID,
		tags,
	)
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
	tags := generateTags("uuid", "testProject")

	nicID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkInterfaces/testNIC"
	err := createVM(
		ctx,
		client,
		"testRG",
		"testVM",
		"eastus",
		nicID,
		30,
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7laRyN...",
		tags,
	)
	if err != nil {
		t.Errorf("createVM failed: %v", err)
	}
}

func TestWaitForSSH(t *testing.T) {
	fmt.Println("Starting TestWaitForSSH")

	// Store the original sshDial function
	originalSSHDial := sshDial
	if originalSSHDial == nil {
		t.Fatal("originalSSHDial is nil")
	}
	fmt.Println("originalSSHDial is not nil")

	defer func() { sshDial = originalSSHDial }()

	// Create a mock sshDial function
	sshDial = func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		fmt.Println("Mock sshDial called")
		return &ssh.Client{}, nil
	}

	err := waitForSSH("1.2.3.4", "testuser", testPrivateKey)
	if err != nil {
		t.Fatalf("waitForSSH failed: %v", err)
	}

	// Test timeout scenario
	sshDial = func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		fmt.Println("Mock sshDial called (timeout scenario)")
		return nil, fmt.Errorf("connection refused")
	}

	sshRetryAttempts = 1
	sshRetryDelay = 0

	err = waitForSSH("1.2.3.4", "testuser", testPrivateKey)
	if err == nil {
		t.Fatal("waitForSSH should have failed with timeout")
	}
}
