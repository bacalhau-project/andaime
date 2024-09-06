package gcp

import (
	"github.com/bacalhau-project/andaime/pkg/models"
)

func ConvertGCPResourceState(state string) models.GCPResourceState {
	switch state {
	case "PROVISIONING", "STAGING":
		return models.GCPResourceStateProvisioning
	case "RUNNING":
		return models.GCPResourceStateRunning
	case "STOPPING", "SUSPENDING":
		return models.GCPResourceStateStopping
	case "TERMINATED", "SUSPENDED":
		return models.GCPResourceStateTerminated
	}

	return models.GCPResourceStateUnknown
}
