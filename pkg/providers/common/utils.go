package common

import (
	"fmt"
)

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

// GenerateTags creates a map of tags for cloud resources
func GenerateTags(projectID, uniqueID string) map[string]string {
	return map[string]string{
		"andaime":                   "true",
		"andaime-id":                uniqueID,
		"andaime-project":           fmt.Sprintf("%s-%s", projectID, uniqueID),
		"unique-id":                 uniqueID,
		"project-id":                projectID,
		"deployed-by":               "andaime",
		"andaime-resource-tracking": "true",
	}
}
