package azure

import (
	"reflect"
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
		dsm.Resources = make(map[string]StateMachineResource, 0)
		dsm.mu.Unlock()
	}()

	// Test updating the status of a resource
	dsm.UpdateStatus(
		"eastus-nsg",
		models.Machine{ID: "eastus-nsg"},
		StateSucceeded,
	)
	if dsm.Resources["eastus-nsg"].State != StateSucceeded {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateSucceeded,
			dsm.Resources["eastus-nsg"].State,
		)
	}

	// Test updating the status of a resource to a lower state
	dsm.UpdateStatus(
		"eastus-nsg",
		models.Machine{ID: "eastus-nsg"},
		StateFailed,
	)
	if dsm.Resources["eastus-nsg"].State != StateFailed {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateFailed,
			dsm.Resources["eastus-nsg"].State,
		)
	}

	// Test updating the status of a resource to a higher state
	dsm.UpdateStatus(
		"eastus-vm",
		models.Machine{ID: "eastus-vm"},
		StateSucceeded,
	)
	if dsm.Resources["eastus-vm"].State != StateSucceeded {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateSucceeded,
			dsm.Resources["eastus-vm"].State,
		)
	}

}

func TestGetStatus(t *testing.T) {
	dsm := GetGlobalStateMachine()
	defer func() {
		dsm.mu.Lock()
		dsm.Resources = make(map[string]StateMachineResource, 0)
		dsm.mu.Unlock()
	}()

	state := StateSucceeded
	// Test getting the status of a resource
	dsm.Resources["123456-vm"] = StateMachineResource{
		Type: reflect.TypeOf(models.Machine{}),
		Resource: models.Machine{
			ID: "123456-vm",
		},
		State: state,
	}
	status, ok := dsm.GetStatus("123456-vm")
	if !ok {
		t.Errorf("Expected the resource to exist, but it doesn't")
	}
	if status != state {
		t.Errorf(
			"Expected state to be %s, but got %s",
			StateSucceeded,
			status,
		)
	}
}

func TestStateProgression(t *testing.T) {
	dsm := GetGlobalStateMachine()
	defer func() {
		dsm.mu.Lock()
		dsm.Resources = make(map[string]StateMachineResource, 0)
		dsm.mu.Unlock()
	}()

	resourceName := "test-resource"
	resource := models.Machine{ID: resourceName}

	states := []ResourceState{
		StateNotStarted,
		StateProvisioning,
		StateSucceeded,
		StateFailed,
	}

	// Test forward progression
	for i, currentState := range states {
		dsm.UpdateStatus(resourceName, resource, currentState)
		status, ok := dsm.GetStatus(resourceName)
		if !ok {
			t.Errorf("Expected resource %s to exist, but it doesn't", resourceName)
		}
		if status != currentState {
			t.Errorf("Expected state to be %s, but got %s", currentState, status)
		}

		// Try to update with all previous states
		for j := 0; j < i; j++ {
			previousState := states[j]
			dsm.UpdateStatus(resourceName, resource, previousState)
			status, _ = dsm.GetStatus(resourceName)
			if status != currentState {
				t.Errorf("State incorrectly changed from %s to %s", currentState, status)
			}
		}
	}

	// Test that final state (StateFailed) cannot be changed
	for _, state := range states {
		dsm.UpdateStatus(resourceName, resource, state)
		status, _ := dsm.GetStatus(resourceName)
		if status != StateFailed {
			t.Errorf("Final state (StateFailed) was incorrectly changed to %s", status)
		}
	}
}
