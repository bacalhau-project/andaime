package azure

import (
	"testing"

	"github.com/bacalhau-project/andaime/pkg/models"
)

// Create a set of sample machines and their statuses
var sampleMachines = []models.Machine{
	{
		ID:       "123456-vm",
		Location: "eastus",
	},
	{
		ID:       "123457-vm",
		Location: "westus",
	},
}

func TestUpdateStatus(t *testing.T) {
	UpdateGlobalDeployment(func(d *models.Deployment) {
		d.Machines = sampleMachines
	})
	dsm := GetGlobalStateMachine()
	defer func() {
		dsm.mu.Lock()
		dsm.statuses = make(map[string]ResourceStatus)
		dsm.mu.Unlock()
	}()

	// Test updating the status of a resource
	dsm.UpdateStatus("vm", "eastus-nsg", StateSucceeded)
	if dsm.statuses["eastus-nsg"].State != StateSucceeded {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateSucceeded,
			dsm.statuses["eastus-nsg"].State,
		)
	}

	// Test updating the status of a resource to a lower state
	dsm.UpdateStatus("vm", "eastus-nsg", StateFailed)
	if dsm.statuses["eastus-nsg"].State != StateFailed {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateFailed,
			dsm.statuses["eastus-nsg"].State,
		)
	}

	// Test updating the status of a resource to a higher state
	dsm.UpdateStatus("vm", "eastus-vm", StateSucceeded)
	if dsm.statuses["eastus-vm"].State != StateSucceeded {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateSucceeded,
			dsm.statuses["eastus-vm"].State,
		)
	}

}

func TestGetStatus(t *testing.T) {
	dsm := GetGlobalStateMachine()
	defer func() {
		dsm.mu.Lock()
		dsm.statuses = make(map[string]ResourceStatus)
		dsm.mu.Unlock()
	}()

	// Test getting the status of a resource
	dsm.statuses["123456-vm"] = ResourceStatus{State: StateSucceeded}
	status, ok := dsm.GetStatus("123456-vm")
	if !ok {
		t.Errorf("Expected the resource to exist, but it doesn't")
	}
	if status != StateSucceeded {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateSucceeded,
			status,
		)
	}

	// Test getting the status of a resource that doesn't exist
	if _, ok := dsm.GetStatus("non-existent-vm"); ok {
		t.Errorf("Expected the resource to not exist, but it does")
	}
}
