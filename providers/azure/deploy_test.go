package azure

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/utils"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTags(t *testing.T) {
	projectID := "test-project"
	uniqueID := utils.GenerateUniqueID()

	tags := generateTags(projectID, uniqueID)

	if tags["project"] == nil || *tags["project"] != "test-project" {
		t.Errorf("Expected project tag to be 'test-project', got %v", tags["project"])
	}

	if tags["deployed_by"] == nil || *tags["deployed_by"] != "andaime" {
		t.Errorf("Expected deployed_by tag to be 'andaime', got %v", tags["deployed_by"])
	}
}

func TestSearchResources(t *testing.T) {
	ctx := context.Background()
	interfaces := getClientInterfaces()
	interfaces.ResourceGraphClient = &MockResourceGraphClient{
		Response: MockQueryResponse(),
	}

	resp, err := searchResources(ctx, interfaces, "testRG", nil, "testSubscription")
	if err != nil {
		t.Errorf("searchResources failed: %v", err)
	}

	assert.NotNil(t, resp, "searchResources returned nil response")
	assert.True(t, *resp.Count == int64(3), "searchResources returned unexpected number of resources")
}
