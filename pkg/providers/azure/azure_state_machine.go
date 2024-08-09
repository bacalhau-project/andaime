package azure

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
)

type ResourceState string

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
	StateNotStarted   ResourceState = "Not Started"
	StateProvisioning ResourceState = "Provisioning"
	StateSucceeded    ResourceState = "Succeeded"
	StateFailed       ResourceState = "Failed"
)

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
	resourceType, resourceKey string,
	state ResourceState,
) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	key := dsm.getDisplayKey(resourceKey, dsm.deployment)
	if key == nil {
		// Handle the case where the key is not found
		return
	}

	currentStatus, exists := dsm.statuses[key.DisplayKey]

	if !exists || currentStatus.State != state {
		dsm.statuses[key.DisplayKey] = ResourceStatus{State: state, LastUpdated: time.Now()}
		disp := display.GetGlobalDisplay()
		disp.UpdateStatus(&models.Status{
			ID:     resourceKey,
			Type:   resourceType,
			Status: string(state),
		})
	}
}

func (dsm *AzureStateMachine) GetStatus(
	resourceType, resourceName string,
) ResourceStatus {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	key := dsm.getDisplayKey(resourceName, dsm.deployment)
	if key == nil {
		// Handle the case where the key is not found
		return ResourceStatus{}
	}

	return dsm.statuses[key.DisplayKey]
}

type DisplayKeyResponse struct {
	DisplayKey string
	IsLocation bool
	Err        error
}

func (dsm *AzureStateMachine) getDisplayKey(
	name string,
	deployment *models.Deployment,
) *DisplayKeyResponse {
	var response DisplayKeyResponse

	// Check if it's a location
	for _, location := range deployment.Locations {
		if strings.HasPrefix(name, location) {
			response.DisplayKey = location
			response.IsLocation = true
			return &response
		}
	}

	// Check if it's a unique machine ID
	for _, machine := range deployment.Machines {
		if strings.HasPrefix(name, machine.ID) {
			response.DisplayKey = machine.ID
			response.IsLocation = false
			return &response
		}
	}

	// If not found, return an error
	response.Err = errors.New("display key not found")
	return &response
}
