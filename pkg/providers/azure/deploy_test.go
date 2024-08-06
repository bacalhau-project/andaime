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
			name: "Valid NSG update",
			deployment: &models.Deployment{
				Machines: []models.Machine{
					{ID: "vm1"},
				},
				NetworkSecurityGroups: make(map[string]*armnetwork.SecurityGroup),
			},
			resource: &armresources.GenericResource{
				Name: utils.ToPtr("nsg-vm1"),
				ID:   utils.ToPtr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkSecurityGroups/nsg-vm1"),
				Type: utils.ToPtr("Microsoft.Network/networkSecurityGroups"),
				Properties: map[string]interface{}{
					"securityRules": []interface{}{
						map[string]interface{}{
							"name":                     "AllowSSH",
							"protocol":                 "Tcp",
							"sourcePortRange":          "*",
							"destinationPortRange":     "22",
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
								Name: utils.ToPtr("AllowSSH"),
								Properties: &armnetwork.SecurityRulePropertiesFormat{
									Protocol:                 utils.ToPtr("Tcp"),
									SourcePortRange:          utils.ToPtr("*"),
									DestinationPortRange:     utils.ToPtr("22"),
									SourceAddressPrefix:      utils.ToPtr("*"),
									DestinationAddressPrefix: utils.ToPtr("*"),
									Access:                   utils.ToPtr("Allow"),
									Priority:                 utils.ToPtr(int32(100)),
									Direction:                utils.ToPtr("Inbound"),
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
