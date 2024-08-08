package azure

import (
	"fmt"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
)

type ResourceState string

const (
	StateNotStarted   ResourceState = "Not Started"
	StateProvisioning ResourceState = "Provisioning"
	StateSucceeded    ResourceState = "Succeeded"
	StateFailed       ResourceState = "Failed"
)

type ResourceStatus struct {
	State       ResourceState
	LastUpdated time.Time
}

type DeploymentStateMachine struct {
	mu       sync.Mutex
	statuses map[string]ResourceStatus
	disp     *display.Display
}

func NewDeploymentStateMachine(disp *display.Display) *DeploymentStateMachine {
	return &DeploymentStateMachine{
		statuses: make(map[string]ResourceStatus),
		disp:     disp,
	}
}

func (dsm *DeploymentStateMachine) UpdateStatus(
	resourceType, resourceName string,
	state ResourceState,
) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	key := fmt.Sprintf("%s-%s", resourceType, resourceName)
	currentStatus, exists := dsm.statuses[key]

	if !exists || currentStatus.State != state {
		dsm.statuses[key] = ResourceStatus{State: state, LastUpdated: time.Now()}
		dsm.disp.UpdateStatus(&models.Status{
			ID:     key,
			Type:   resourceType,
			Status: string(state),
		})
	}
}

func (dsm *DeploymentStateMachine) GetStatus(resourceType, resourceName string) ResourceStatus {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	key := fmt.Sprintf("%s-%s", resourceType, resourceName)
	return dsm.statuses[key]
}
