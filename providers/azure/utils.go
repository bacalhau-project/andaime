package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// generateTags creates a map of tags for Azure resources
func generateTags(uniqueID, projectID string) map[string]*string {
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
