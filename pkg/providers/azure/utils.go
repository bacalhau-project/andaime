package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// generateTags creates a map of tags for Azure resources
func generateTags(projectID, uniqueID string) map[string]*string {
	return map[string]*string{
		"andaime":                   to.Ptr("true"),
		"andaime-id":                to.Ptr(uniqueID),
		"andaime-project":           to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID)),
		"unique-id":                 to.Ptr(uniqueID),
		"project-id":                to.Ptr(projectID),
		"deployed-by":               to.Ptr("andaime"),
		"andaime-resource-tracking": to.Ptr("true"),
	}
}

func EnsureTags(tags map[string]*string, projectID, uniqueID string) map[string]*string {
	if tags == nil {
		tags = map[string]*string{}
	}
	if tags["andaime"] == nil {
		tags["andaime"] = to.Ptr("true")
	}
	if tags["deployed-by"] == nil {
		tags["deployed-by"] = to.Ptr("andaime")
	}
	if tags["andaime-resource-tracking"] == nil {
		tags["andaime-resource-tracking"] = to.Ptr("true")
	}
	if tags["unique-id"] == nil {
		tags["unique-id"] = to.Ptr(uniqueID)
	}
	if tags["project-id"] == nil {
		tags["project-id"] = to.Ptr(projectID)
	}
	if tags["andaime-project"] == nil {
		tags["andaime-project"] = to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID))
	}
	return tags
}

func IsValidResourceGroupName(name string) bool {
	// Resource group name must be 1-90 characters long and can only contain alphanumeric characters,
	// underscores, parentheses, hyphens, and periods (except at end)
	if len(name) < 1 || len(name) > 90 {
		return false
	}
	for i, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '(' ||
			char == ')' || char == '-' || (char == '.' && i != len(name)-1)) {
			return false
		}
	}
	return true
}
