package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
)

// generateTags creates a map of tags for Azure resources
func GenerateTags(projectID, uniqueID string) map[string]*string {
	commonTags := common.GenerateTags(projectID, uniqueID)
	azureTags := make(map[string]*string)
	for k, v := range commonTags {
		azureTags[k] = to.Ptr(v)
	}
	return azureTags
}

// Keep any Azure-specific utility functions here, if any
