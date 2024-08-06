package azure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
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

// generateTags creates a map of tags for Azure resources
func GenerateTags(projectID, uniqueID string) map[string]*string {
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

func isQuotaExceededError(err error) bool {
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) {
		return azErr.ErrorCode == "InvalidTemplateDeployment" &&
			strings.Contains(azErr.Error(), "ResourceCountExceedsLimitDueToTemplate") &&
			strings.Contains(azErr.Error(), "PublicIpAddress")
	}
	return false
}
