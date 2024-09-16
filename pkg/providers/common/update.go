// pkg/providers/common/update.go
package common

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
)

// UpdateType represents the type of update action.
type UpdateType string

const (
	UpdateTypeResource UpdateType = "resource"
	UpdateTypeService  UpdateType = "service"
	UpdateTypeComplete UpdateType = "complete"
)

// UpdatePayload holds the data for an update action.
type UpdatePayload struct {
	UpdateType    UpdateType
	ServiceType   models.ServiceType
	ServiceState  models.ServiceState
	ResourceType  models.ResourceType
	ResourceState models.MachineResourceState
	Complete      bool
}

// String returns a string representation of the UpdatePayload.
func (u *UpdatePayload) String() string {
	if u.UpdateType == UpdateTypeResource {
		return fmt.Sprintf("%s: %s", u.UpdateType, u.ResourceType.ResourceString)
	}
	return fmt.Sprintf("%s: %s", u.UpdateType, u.ServiceType.Name)
}

// UpdateAction represents an action to update a machine's state.
type UpdateAction struct {
	MachineName string
	UpdateData  UpdatePayload
	UpdateFunc  func(models.Machiner, UpdatePayload)
}

// NewUpdateAction creates a new UpdateAction instance.
func NewUpdateAction(machineName string, updateData UpdatePayload) UpdateAction {
	l := logger.Get()
	updateFunc := func(machine models.Machiner, update UpdatePayload) {
		switch update.UpdateType {
		case UpdateTypeResource:
			machine.SetMachineResourceState(
				update.ResourceType.ResourceString,
				update.ResourceState,
			)
		case UpdateTypeService:
			machine.SetServiceState(update.ServiceType.Name, update.ServiceState)
		default:
			l.Errorf("Invalid update type: %s", update.UpdateType)
		}
	}
	return UpdateAction{
		MachineName: machineName,
		UpdateData:  updateData,
		UpdateFunc:  updateFunc,
	}
}
