package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerateTags(t *testing.T) {
	tagsToTest := map[string]string{"project-id": "test-project",
		"unique-id": utils.GenerateUniqueID()}

	tags := GenerateTags(tagsToTest["project-id"], tagsToTest["unique-id"])

	if tags == nil {
		t.Error("generateTags returned nil tags")
	}

	for key, value := range tagsToTest {
		if *tags[key] != value {
			t.Errorf("Expected tag %s to be %s, got %s", key, value, *tags[key])
		}
	}
}

// func TestUpdateDiskStatus(t *testing.T) {
// 	tests := []struct {
// 		name           string
// 		deployment     *models.Deployment
// 		resource       *armcompute.Disk
// 		expectedResult map[string]*models.Disk
// 	}{
// 		{
// 			name: "Valid Disk update",
// 			deployment: &models.Deployment{
// 				Disks: make(map[string]*models.Disk),
// 			},
// 			resource: &armcompute.Disk{
// 				Name: utils.ToPtr("disk1"),
// 				ID: utils.ToPtr(
// 					"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk1",
// 				),
// 				Type: utils.ToPtr("Microsoft.Compute/disks"),
// 				Properties: &armcompute.DiskProperties{
// 					DiskSizeGB: utils.ToPtr(int32(128)),
// 					DiskState:  utils.ToPtr(armcompute.DiskStateAttached),
// 				},
// 			},
// 			expectedResult: map[string]*models.Disk{
// 				"disk1": {
// 					Name:   "disk1",
// 					ID:     "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk1",
// 					SizeGB: 128,
// 					State:  armcompute.DiskStateAttached,
// 				},
// 			},
// 		},
// 		{
// 			name: "Invalid Disk properties",
// 			deployment: &models.Deployment{
// 				Disks: make(map[string]*models.Disk),
// 			},
// 			resource: &armcompute.Disk{
// 				Name: utils.ToPtr("disk2"),
// 				ID: utils.ToPtr(
// 					"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk2",
// 				),
// 				Type:       utils.ToPtr("Microsoft.Compute/disks"),
// 				Properties: &armcompute.DiskProperties{},
// 			},
// 			expectedResult: map[string]*models.Disk{
// 				"disk2": {
// 					Name: "disk2",
// 					ID:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk2",
// 				},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			provider := &AzureProvider{}
// 			provider.updateDiskStatus(tt.deployment, tt.resource)

// 			assert.Equal(t, tt.expectedResult, tt.deployment.Disks)
// 		})
// 	}
// }

// func TestUpdateVNetStatus(t *testing.T) {
// 	tests := []struct {
// 		name           string
// 		deployment     *models.Deployment
// 		resource       *armnetwork.VirtualNetwork
// 		expectedResult map[string][]*armnetwork.Subnet
// 	}{
// 		{
// 			name: "Valid VNet update",
// 			deployment: &models.Deployment{
// 				Subnets: make(map[string][]*armnetwork.Subnet),
// 			},
// 			resource: &armnetwork.VirtualNetwork{
// 				Name: utils.ToPtr("vnet1"),
// 				Type: utils.ToPtr("Microsoft.Network/virtualNetworks"),
// 				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
// 					Subnets: []*armnetwork.Subnet{
// 						{
// 							Name: utils.ToPtr("subnet1"),
// 							ID: utils.ToPtr(
// 								"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet1",
// 							),
// 							Properties: &armnetwork.SubnetPropertiesFormat{
// 								AddressPrefix: utils.ToPtr("10.0.1.0/24"),
// 							},
// 						},
// 						{
// 							Name: utils.ToPtr("subnet2"),
// 							ID: utils.ToPtr(
// 								"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet2",
// 							),
// 							Properties: &armnetwork.SubnetPropertiesFormat{
// 								AddressPrefix: utils.ToPtr("10.0.2.0/24"),
// 							},
// 						},
// 					},
// 				},
// 			},
// 			expectedResult: map[string][]*armnetwork.Subnet{
// 				"vnet1": {
// 					{
// 						Name: utils.ToPtr("subnet1"),
// 						ID: utils.ToPtr(
// 							"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet1",
// 						),
// 						Properties: &armnetwork.SubnetPropertiesFormat{
// 							AddressPrefix: utils.ToPtr("10.0.1.0/24"),
// 						},
// 					},
// 					{
// 						Name: utils.ToPtr("subnet2"),
// 						ID: utils.ToPtr(
// 							"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet2",
// 						),
// 						Properties: &armnetwork.SubnetPropertiesFormat{
// 							AddressPrefix: utils.ToPtr("10.0.2.0/24"),
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			name: "No subnets in VNet",
// 			deployment: &models.Deployment{
// 				Subnets: make(map[string][]*armnetwork.Subnet),
// 			},
// 			resource: &armnetwork.VirtualNetwork{
// 				Name: utils.ToPtr("vnet2"),
// 				Type: utils.ToPtr("Microsoft.Network/virtualNetworks"),
// 				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
// 					Subnets: []*armnetwork.Subnet{},
// 				},
// 			},
// 			expectedResult: map[string][]*armnetwork.Subnet{},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			provider := &AzureProvider{}
// 			provider.updateVNetStatus(tt.deployment, tt.resource)

// 			assert.Equal(t, tt.expectedResult, tt.deployment.Subnets)
// 		})
// 	}
// }
// func TestUpdateNICStatus(t *testing.T) {
// 	tests := []struct {
// 		name           string
// 		deployment     *models.Deployment
// 		resource       *armnetwork.Interface
// 		expectedResult []models.Machine
// 	}{
// 		{
// 			name: "Valid NIC update",
// 			deployment: &models.Deployment{
// 				Machines: []models.Machine{
// 					{ID: "vm1", PrivateIP: ""},
// 					{ID: "vm2", PrivateIP: ""},
// 				},
// 			},
// 			resource: &armnetwork.Interface{
// 				Name: utils.ToPtr("nic-vm1"),
// 				Type: utils.ToPtr("Microsoft.Network/networkInterfaces"),
// 				Properties: &armnetwork.InterfacePropertiesFormat{
// 					IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
// 						{
// 							Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
// 								PrivateIPAddress: utils.ToPtr("10.0.0.4"),
// 							},
// 						},
// 					},
// 				},
// 			},
// 			expectedResult: []models.Machine{
// 				{ID: "vm1", PrivateIP: "10.0.0.4"},
// 				{ID: "vm2", PrivateIP: ""},
// 			},
// 		},
// 		{
// 			name: "No matching machine",
// 			deployment: &models.Deployment{
// 				Machines: []models.Machine{
// 					{ID: "vm1", PrivateIP: ""},
// 					{ID: "vm2", PrivateIP: ""},
// 				},
// 			},
// 			resource: &armnetwork.Interface{
// 				Name: utils.ToPtr("nic-vm3"),
// 				Type: utils.ToPtr("Microsoft.Network/networkInterfaces"),
// 				Properties: &armnetwork.InterfacePropertiesFormat{
// 					IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
// 						{
// 							Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
// 								PrivateIPAddress: utils.ToPtr("10.0.0.5"),
// 							},
// 						},
// 					},
// 				},
// 			},
// 			expectedResult: []models.Machine{
// 				{ID: "vm1", PrivateIP: ""},
// 				{ID: "vm2", PrivateIP: ""},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			provider := &AzureProvider{}
// 			provider.updateNICStatus(tt.deployment, tt.resource)

// 			assert.Equal(t, tt.expectedResult, tt.deployment.Machines)
// 		})
// 	}
// }

// func TestUpdateNSGStatus(t *testing.T) {
// 	tests := []struct {
// 		name           string
// 		deployment     *models.Deployment
// 		resource       *armnetwork.SecurityGroup
// 		expectedResult map[string]*armnetwork.SecurityGroup
// 	}{
// 		{
// 			name: "Valid NSG update with allowed ports",
// 			deployment: &models.Deployment{
// 				Machines: []models.Machine{
// 					{ID: "vm1", Location: "eastus"},
// 				},
// 				NetworkSecurityGroups: make(map[string]*armnetwork.SecurityGroup),
// 				AllowedPorts:          []int{22, 80, 443, 8080},
// 			},
// 			resource: &armnetwork.SecurityGroup{
// 				Name:     utils.ToPtr("nsg-vm1"),
// 				Location: utils.ToPtr("eastus"),
// 				ID: utils.ToPtr(
// 					"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm1",
// 				),
// 				Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
// 				Properties: &armnetwork.SecurityGroupPropertiesFormat{
// 					SecurityRules: []*armnetwork.SecurityRule{
// 						{
// 							Name: utils.ToPtr("ExistingRule"),
// 							Properties: &armnetwork.SecurityRulePropertiesFormat{
// 								Protocol: utils.ToPtr(
// 									armnetwork.SecurityRuleProtocolTCP,
// 								),
// 								SourcePortRange:          utils.ToPtr("*"),
// 								DestinationPortRange:     utils.ToPtr("8080"),
// 								SourceAddressPrefix:      utils.ToPtr("*"),
// 								DestinationAddressPrefix: utils.ToPtr("*"),
// 								Access: utils.ToPtr(
// 									armnetwork.SecurityRuleAccessAllow,
// 								),
// 								Priority: utils.ToPtr(int32(100)),
// 								Direction: utils.ToPtr(
// 									armnetwork.SecurityRuleDirectionInbound,
// 								),
// 							},
// 						},
// 					},
// 				},
// 			},
// 			expectedResult: map[string]*armnetwork.SecurityGroup{
// 				"nsg-vm1": {
// 					Name:     utils.ToPtr("nsg-vm1"),
// 					Location: utils.ToPtr("eastus"),
// 					ID: utils.ToPtr(
// 						"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm1",
// 					),
// 					Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
// 					Properties: &armnetwork.SecurityGroupPropertiesFormat{
// 						SecurityRules: []*armnetwork.SecurityRule{
// 							{
// 								Name: utils.ToPtr("ExistingRule"),
// 								Properties: &armnetwork.SecurityRulePropertiesFormat{
// 									Protocol: utils.ToPtr(
// 										armnetwork.SecurityRuleProtocolTCP,
// 									),
// 									SourcePortRange:          utils.ToPtr("*"),
// 									DestinationPortRange:     utils.ToPtr("8080"),
// 									SourceAddressPrefix:      utils.ToPtr("*"),
// 									DestinationAddressPrefix: utils.ToPtr("*"),
// 									Access: utils.ToPtr(
// 										armnetwork.SecurityRuleAccessAllow,
// 									),
// 									Priority: utils.ToPtr(int32(100)),
// 									Direction: utils.ToPtr(
// 										armnetwork.SecurityRuleDirectionInbound,
// 									),
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			name: "No security rules",
// 			deployment: &models.Deployment{
// 				Machines: []models.Machine{
// 					{ID: "vm2", Location: "westus"},
// 				},
// 				NetworkSecurityGroups: make(map[string]*armnetwork.SecurityGroup),
// 				AllowedPorts:          []int{22, 80, 443},
// 			},
// 			resource: &armnetwork.SecurityGroup{
// 				Name:     utils.ToPtr("nsg-vm2"),
// 				Location: utils.ToPtr("westus"),
// 				ID: utils.ToPtr(
// 					"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm2",
// 				),
// 				Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
// 				Properties: &armnetwork.SecurityGroupPropertiesFormat{
// 					SecurityRules: []*armnetwork.SecurityRule{},
// 				},
// 			},
// 			expectedResult: map[string]*armnetwork.SecurityGroup{
// 				"nsg-vm2": {
// 					Name:     utils.ToPtr("nsg-vm2"),
// 					Location: utils.ToPtr("westus"),
// 					ID: utils.ToPtr(
// 						"/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm2",
// 					),
// 					Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
// 					Properties: &armnetwork.SecurityGroupPropertiesFormat{
// 						SecurityRules: []*armnetwork.SecurityRule{},
// 					},
// 				},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			mockClient := &MockAzureClient{}
// 			provider := &AzureProvider{
// 				Client: mockClient,
// 				Config: viper.New(),
// 			}
// 			provider.updateNSGStatus(tt.deployment, tt.resource)

// 			assert.Equal(t, tt.expectedResult, tt.deployment.NetworkSecurityGroups)
// 		})
// 	}
// }

func TestPrepareResourceGroup_NoMachines(t *testing.T) {
	// Arrange
	ctx := context.Background()
	UpdateGlobalDeployment(func(d *models.Deployment) { d.Machines = []models.Machine{} })

	provider := &AzureProvider{
		Client: &MockAzureClient{},
	}

	// Act
	err := provider.PrepareResourceGroup(ctx)

	// Assert
	assert.EqualError(
		t,
		err,
		"resource group location is not set and couldn't be inferred from machines",
	)
}

func TestPrepareResourceGroup_GetOrCreateResourceGroupError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	UpdateGlobalDeployment(func(d *models.Deployment) {
		d.Machines = []models.Machine{
			{Location: "westus2"},
		}
		d.ResourceGroupName = "test-rg"
	})

	mockClient := &MockAzureClient{}
	mockClient.On("GetOrCreateResourceGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&armresources.ResourceGroup{}, fmt.Errorf("failed to create resource group"))
	provider := &AzureProvider{
		Client: mockClient,
	}

	// Act
	err := provider.PrepareResourceGroup(ctx)

	// Assert
	assert.EqualError(t, err, "failed to create resource group: failed to create resource group")
	mockClient.AssertExpectations(t)
}
package azure

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

func TestChannelClosing(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Create a mock AzureProvider
	provider := &AzureProvider{}

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the display
	disp := display.GetGlobalDisplay()
	go disp.Start()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			// Simulate deployment process
			time.Sleep(100 * time.Millisecond)
			disp.UpdateStatus(&models.Status{
				ID:     m.Name,
				Status: "Deployed",
			})
		}(machine)
	}

	// Wait for all deployments to finish
	wg.Wait()

	// Stop the display
	disp.Stop()

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after deployment")
	}
}

func TestCancelledDeployment(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Create a mock AzureProvider
	provider := &AzureProvider{}

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the display
	disp := display.GetGlobalDisplay()
	go disp.Start()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment with cancellation
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Cancelled",
				})
			case <-time.After(500 * time.Millisecond):
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Deployed",
				})
			}
		}(machine)
	}

	// Cancel the deployment after a short delay
	time.AfterFunc(200*time.Millisecond, cancel)

	// Wait for all deployments to finish or be cancelled
	wg.Wait()

	// Stop the display
	disp.Stop()

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after cancelled deployment")
	}
}
package azure

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

func TestChannelClosing(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Create a mock AzureProvider
	provider := &AzureProvider{}

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the display
	disp := display.GetGlobalDisplay()
	go disp.Start()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			// Simulate deployment process
			time.Sleep(100 * time.Millisecond)
			disp.UpdateStatus(&models.Status{
				ID:     m.Name,
				Status: "Deployed",
			})
		}(machine)
	}

	// Wait for all deployments to finish
	wg.Wait()

	// Stop the display
	disp.Stop()

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after deployment")
	}
}

func TestCancelledDeployment(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Create a mock AzureProvider
	provider := &AzureProvider{}

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the display
	disp := display.GetGlobalDisplay()
	go disp.Start()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment with cancellation
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Cancelled",
				})
			case <-time.After(500 * time.Millisecond):
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Deployed",
				})
			}
		}(machine)
	}

	// Cancel the deployment after a short delay
	time.AfterFunc(200*time.Millisecond, cancel)

	// Wait for all deployments to finish or be cancelled
	wg.Wait()

	// Stop the display
	disp.Stop()

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after cancelled deployment")
	}
}
