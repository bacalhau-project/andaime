package azure

import (
	"testing"

	"github.com/bacalhau-project/andaime/utils"
)

func TestGenerateTags(t *testing.T) {
	tagsToTest := map[string]string{"project-id": "test-project",
		"unique-id": utils.GenerateUniqueID()}

	tags := generateTags(tagsToTest["project-id"], tagsToTest["unique-id"])

	if tags == nil {
		t.Error("generateTags returned nil tags")
	}

	for key, value := range tagsToTest {
		if *tags[key] != value {
			t.Errorf("Expected tag %s to be %s, got %s", key, value, *tags[key])
		}
	}
}
