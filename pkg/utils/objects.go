package utils

import (
	"github.com/bacalhau-project/andaime/pkg/models"
)

func UpdateStatus(status *models.Status, newStatus *models.Status) *models.Status {
	if newStatus.DetailedStatus != "" {
		status.DetailedStatus = newStatus.DetailedStatus
	}
	if newStatus.PublicIP != "" {
		status.PublicIP = newStatus.PublicIP
	}
	if newStatus.PrivateIP != "" {
		status.PrivateIP = newStatus.PrivateIP
	}
	if newStatus.InstanceID != "" {
		status.InstanceID = newStatus.InstanceID
	}
	if newStatus.Location != "" {
		status.Location = newStatus.Location
	}

	return status
}
