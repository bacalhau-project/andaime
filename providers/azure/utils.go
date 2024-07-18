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

type DefaultSSHWaiter struct{}

func (w *DefaultSSHWaiter) WaitForSSH(publicIP, username string, privateKey []byte) error {
	return waitForSSH(publicIP, username, privateKey)
}

func ensureTags(tags map[string]*string, projectID, uniqueID string) {
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
}
