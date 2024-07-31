package utils

import (
	"time"

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
