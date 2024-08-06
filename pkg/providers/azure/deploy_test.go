package azure

import (
	"testing"

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
