package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/stretchr/testify/assert"
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

func TestUpdateDiskStatus(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *models.Deployment
		resource       *armresources.GenericResource
		expectedResult map[string]*models.Disk
	}{
		{
			name: "Valid Disk update",
			deployment: &models.Deployment{
				Disks: make(map[string]*models.Disk),
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("disk1"),
				ID:   utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk1"),
				Type: utils.ToPtr("Microsoft.Compute/disks"),
				Properties: map[string]interface{}{
					"diskSizeGB": float64(128),
					"diskState":  "Attached",
				},
			},
			expectedResult: map[string]*models.Disk{
				"disk1": {
					Name:   "disk1",
					ID:     "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk1",
					SizeGB: 128,
					State:  "Attached",
				},
			},
		},
		{
			name: "Invalid Disk properties",
			deployment: &models.Deployment{
				Disks: make(map[string]*models.Disk),
			},
			resource: &armresources.GenericResource{
				Name:       utils.ToPtr("disk2"),
				ID:         utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/disk2"),
				Type:       utils.ToPtr("Microsoft.Compute/disks"),
				Properties: map[string]interface{}{},
			},
			expectedResult: map[string]*models.Disk{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &AzureProvider{}
			provider.updateDiskStatus(tt.deployment, tt.resource)

			assert.Equal(t, tt.expectedResult, tt.deployment.Disks)
		})
	}
}

func TestUpdateVNetStatus(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *models.Deployment
		resource       *armresources.GenericResource
		expectedResult map[string][]*armnetwork.Subnet
	}{
		{
			name: "Valid VNet update",
			deployment: &models.Deployment{
				Subnets: make(map[string][]*armnetwork.Subnet),
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("vnet1"),
				Type: utils.ToPtr("Microsoft.Network/virtualNetworks"),
				Properties: map[string]interface{}{
					"subnets": []interface{}{
						map[string]interface{}{
							"name":          "subnet1",
							"id":            "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet1",
							"addressPrefix": "10.0.1.0/24",
						},
						map[string]interface{}{
							"name":          "subnet2",
							"id":            "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet2",
							"addressPrefix": "10.0.2.0/24",
						},
					},
				},
			},
			expectedResult: map[string][]*armnetwork.Subnet{
				"vnet1": {
					{
						Name: utils.ToPtr("subnet1"),
						ID:   utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet1"),
						Properties: &armnetwork.SubnetPropertiesFormat{
							AddressPrefix: utils.ToPtr("10.0.1.0/24"),
						},
					},
					{
						Name: utils.ToPtr("subnet2"),
						ID:   utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet2"),
						Properties: &armnetwork.SubnetPropertiesFormat{
							AddressPrefix: utils.ToPtr("10.0.2.0/24"),
						},
					},
				},
			},
		},
		{
			name: "No subnets in VNet",
			deployment: &models.Deployment{
				Subnets: make(map[string][]*armnetwork.Subnet),
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("vnet2"),
				Type: utils.ToPtr("Microsoft.Network/virtualNetworks"),
				Properties: map[string]interface{}{
					"subnets": []interface{}{},
				},
			},
			expectedResult: map[string][]*armnetwork.Subnet{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &AzureProvider{}
			provider.updateVNetStatus(tt.deployment, tt.resource)

			assert.Equal(t, tt.expectedResult, tt.deployment.Subnets)
		})
	}
}
func TestUpdateNICStatus(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *models.Deployment
		resource       *armresources.GenericResource
		expectedResult []models.Machine
	}{
		{
			name: "Valid NIC update",
			deployment: &models.Deployment{
				Machines: []models.Machine{
					{ID: "vm1", PrivateIP: ""},
					{ID: "vm2", PrivateIP: ""},
				},
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("nic-vm1"),
				Type: utils.ToPtr("Microsoft.Network/networkInterfaces"),
				Properties: map[string]interface{}{
					"ipConfigurations": []interface{}{
						map[string]interface{}{
							"privateIPAddress": "10.0.0.4",
						},
					},
				},
			},
			expectedResult: []models.Machine{
				{ID: "vm1", PrivateIP: "10.0.0.4"},
				{ID: "vm2", PrivateIP: ""},
			},
		},
		{
			name: "No matching machine",
			deployment: &models.Deployment{
				Machines: []models.Machine{
					{ID: "vm1", PrivateIP: ""},
					{ID: "vm2", PrivateIP: ""},
				},
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("nic-vm3"),
				Type: utils.ToPtr("Microsoft.Network/networkInterfaces"),
				Properties: map[string]interface{}{
					"ipConfigurations": []interface{}{
						map[string]interface{}{
							"privateIPAddress": "10.0.0.5",
						},
					},
				},
			},
			expectedResult: []models.Machine{
				{ID: "vm1", PrivateIP: ""},
				{ID: "vm2", PrivateIP: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &AzureProvider{}
			provider.updateNICStatus(tt.deployment, tt.resource)

			assert.Equal(t, tt.expectedResult, tt.deployment.Machines)
		})
	}
}

func TestUpdateNSGStatus(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *models.Deployment
		resource       *armresources.GenericResource
		expectedResult map[string]*armnetwork.SecurityGroup
	}{
		{
			name: "Valid NSG update with allowed ports",
			deployment: &models.Deployment{
				Machines: []models.Machine{
					{ID: "vm1"},
				},
				NetworkSecurityGroups: make(map[string]*armnetwork.SecurityGroup),
				AllowedPorts:          []int{22, 80, 443},
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("nsg-vm1"),
				ID:   utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm1"),
				Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
				Properties: map[string]interface{}{
					"securityRules": []interface{}{
						map[string]interface{}{
							"name":                     "ExistingRule",
							"protocol":                 "Tcp",
							"sourcePortRange":          "*",
							"destinationPortRange":     "8080",
							"sourceAddressPrefix":      "*",
							"destinationAddressPrefix": "*",
							"access":                   "Allow",
							"priority":                 float64(100),
							"direction":                "Inbound",
						},
					},
				},
			},
			expectedResult: map[string]*armnetwork.SecurityGroup{
				"vm1": {
					Name: utils.ToPtr("nsg-vm1"),
					ID:   utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm1"),
					Properties: &armnetwork.SecurityGroupPropertiesFormat{
						SecurityRules: []*armnetwork.SecurityRule{
							{
								Name: utils.ToPtr("ExistingRule"),
								Properties: &armnetwork.SecurityRulePropertiesFormat{
									Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("Tcp")),
									SourcePortRange:          utils.ToPtr("*"),
									DestinationPortRange:     utils.ToPtr("8080"),
									SourceAddressPrefix:      utils.ToPtr("*"),
									DestinationAddressPrefix: utils.ToPtr("*"),
									Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
									Priority:                 utils.ToPtr(int32(100)),
									Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
								},
							},
							{
								Name: utils.ToPtr("AllowPort22"),
								Properties: &armnetwork.SecurityRulePropertiesFormat{
									Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("Tcp")),
									SourcePortRange:          utils.ToPtr("*"),
									DestinationPortRange:     utils.ToPtr("22"),
									SourceAddressPrefix:      utils.ToPtr("*"),
									DestinationAddressPrefix: utils.ToPtr("*"),
									Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
									Priority:                 utils.ToPtr(int32(1000)),
									Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
								},
							},
							{
								Name: utils.ToPtr("AllowPort80"),
								Properties: &armnetwork.SecurityRulePropertiesFormat{
									Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("Tcp")),
									SourcePortRange:          utils.ToPtr("*"),
									DestinationPortRange:     utils.ToPtr("80"),
									SourceAddressPrefix:      utils.ToPtr("*"),
									DestinationAddressPrefix: utils.ToPtr("*"),
									Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
									Priority:                 utils.ToPtr(int32(1001)),
									Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
								},
							},
							{
								Name: utils.ToPtr("AllowPort443"),
								Properties: &armnetwork.SecurityRulePropertiesFormat{
									Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("Tcp")),
									SourcePortRange:          utils.ToPtr("*"),
									DestinationPortRange:     utils.ToPtr("443"),
									SourceAddressPrefix:      utils.ToPtr("*"),
									DestinationAddressPrefix: utils.ToPtr("*"),
									Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
									Priority:                 utils.ToPtr(int32(1002)),
									Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "No matching machine",
			deployment: &models.Deployment{
				Machines: []models.Machine{
					{ID: "vm1"},
				},
				NetworkSecurityGroups: make(map[string]*armnetwork.SecurityGroup),
				AllowedPorts:          []int{22, 80, 443},
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("nsg-vm2"),
				Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
				Properties: map[string]interface{}{
					"securityRules": []interface{}{
						map[string]interface{}{
							"name": "AllowHTTP",
						},
					},
				},
			},
			expectedResult: map[string]*armnetwork.SecurityGroup{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &AzureProvider{}
			provider.updateNSGStatus(tt.deployment, tt.resource)

			assert.Equal(t, tt.expectedResult, tt.deployment.NetworkSecurityGroups)
		})
	}
}
