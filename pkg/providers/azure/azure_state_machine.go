package azure

import (
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
)

type ResourceState int

var (
	globalStateMachine   *AzureStateMachine
	globalStateMachineMu sync.Mutex
)

func GetGlobalStateMachine() *AzureStateMachine {
	globalStateMachineMu.Lock()
	defer globalStateMachineMu.Unlock()

	if globalStateMachine == nil {
		deployment := GetGlobalDeployment()
		globalStateMachine = NewDeploymentStateMachine(deployment)
	}

	return globalStateMachine
}

const (
	StateNotStarted ResourceState = iota
	StateProvisioning
	StateSucceeded
	StateFailed
)

// String method to convert ResourceState to string for display purposes
func (rs ResourceState) String() string {
	return [...]string{"Not Started", "Provisioning", "Succeeded", "Failed"}[rs]
}

type ResourceStatus struct {
	State       ResourceState
	LastUpdated time.Time
}

type AzureStateMachine struct {
	mu         sync.Mutex
	statuses   map[string]ResourceStatus
	deployment *models.Deployment
}

func NewDeploymentStateMachine(deployment *models.Deployment) *AzureStateMachine {
	return &AzureStateMachine{
		statuses:   make(map[string]ResourceStatus),
		deployment: deployment,
	}
}

func (dsm *AzureStateMachine) UpdateStatus(
	resourceType, resourceName string,
	state ResourceState,
) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()
	disp := display.GetGlobalDisplay()

	if dsm.statuses[resourceName].State > state {
		return
	}

	dsm.statuses[resourceName] = ResourceStatus{State: state, LastUpdated: time.Now()}

	// The only thing we care about is the portion of the string before
	// the first "-"
	stub := strings.Split(resourceName, "-")[0]

	// We'll identify it's a location by looking through all the locations and matching
	// to the key.
	isLocation := false
	for _, machine := range dsm.deployment.Machines {
		if machine.Location == stub {
			isLocation = true
			disp.UpdateStatus(&models.Status{
				ID:     machine.Name,
				Status: state.String(),
			})
		}
	}
	if !isLocation {
		return
	}

	// Otherwise, it's not a location, so we'll just update the status for the specific machine
	for _, machine := range dsm.deployment.Machines {
		if machine.ID == stub {
			disp.UpdateStatus(&models.Status{
				ID:     machine.Name,
				Status: state.String(),
			})
		}
	}
}

// Write a GetStatus method that returns the status of the deployment
func (dsm *AzureStateMachine) GetStatus(key string) (ResourceState, bool) {
	status, ok := dsm.statuses[key]
	return status.State, ok
}
