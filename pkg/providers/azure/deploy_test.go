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
	// Create a mock AzureProvider
	provider := &AzureProvider{}

	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "vm1", PrivateIP: ""},
			{ID: "vm2", PrivateIP: ""},
		},
	}

	// Create a mock resource
	resourceName := "nic-vm1"
	resourceType := "Microsoft.Network/networkInterfaces"
	resource := &armresources.GenericResource{
		Name: &resourceName,
		Type: &resourceType,
		Properties: map[string]interface{}{
			"ipConfigurations": []interface{}{
				map[string]interface{}{
					"privateIPAddress": "10.0.0.4",
				},
			},
		},
	}

	// Call the updateNICStatus function
	provider.updateNICStatus(deployment, resource)

	// Assert that the private IP was updated for the correct machine
	assert.Equal(t, "10.0.0.4", deployment.Machines[0].PrivateIP)
	assert.Equal(t, "", deployment.Machines[1].PrivateIP)
}
