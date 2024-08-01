package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/bacalhau-project/andaime/pkg/models"
)

func UpdateStatus(status *models.Status, newStatus *models.Status) *models.Status {
	// l := logger.Get()
	// l.Debugf("Updating status: %+v", newStatus)
	// l.Debugf("Current status: %+v", status)
	if newStatus.Status != "" {
		status.Status = newStatus.Status
	}

	if newStatus.DetailedStatus != "" {
		// l.Debugf(
		// 	"Updating detailed status from %s to %s",
		// 	status.DetailedStatus,
		// 	newStatus.DetailedStatus,
		// )
		status.DetailedStatus = newStatus.DetailedStatus
	}
	if newStatus.PublicIP != "" {
		// l.Debugf(
		// 	"Updating public IP from %s to %s",
		// 	status.PublicIP,
		// 	newStatus.PublicIP,
		// )
		status.PublicIP = newStatus.PublicIP
	}
	if newStatus.PrivateIP != "" {
		// l.Debugf(
		// 	"Updating private IP from %s to %s",
		// 	status.PrivateIP,
		// 	newStatus.PrivateIP,
		// )
		status.PrivateIP = newStatus.PrivateIP
	}
	if newStatus.InstanceID != "" {
		// l.Debugf(
		// 	"Updating instance ID from %s to %s",
		// 	status.InstanceID,
		// 	newStatus.InstanceID,
		// )
		status.InstanceID = newStatus.InstanceID
	}
	if newStatus.Location != "" {
		// l.Debugf(
		// 	"Updating location from %s to %s",
		// 	status.Location,
		// 	newStatus.Location,
		// )
		status.Location = newStatus.Location
	}

	if status.StartTime.IsZero() {
		status.StartTime = time.Now()
	}

	status.ElapsedTime = newStatus.ElapsedTime

	return status
}

func EnsureAzureTags(tags map[string]*string, projectID, uniqueID string) map[string]*string {
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

func StructToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
