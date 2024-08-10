package azure

import (
	"reflect"
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

const (
	StateNotStarted ResourceState = iota
	StateProvisioning
	StateSucceeded
	StateFailed
)

type StateMachineResource struct {
	Type        reflect.Type
	Resource    interface{}
	State       ResourceState
	LastUpdated time.Time
}

type AzureStateMachine struct {
	mu        sync.Mutex
	Resources map[string]StateMachineResource
}

func GetGlobalStateMachine() *AzureStateMachine {
	globalStateMachineMu.Lock()
	defer globalStateMachineMu.Unlock()

	if globalStateMachine == nil {
		deployment := GetGlobalDeployment()
		globalStateMachine = NewDeploymentStateMachine(deployment)
	}

	return globalStateMachine
}

func (rs ResourceState) String() string {
	return [...]string{"Not Started", "Provisioning", "Succeeded", "Failed"}[rs]
}

func NewDeploymentStateMachine(deployment *models.Deployment) *AzureStateMachine {
	return &AzureStateMachine{
		Resources: make(map[string]StateMachineResource),
	}
}

func (dsm *AzureStateMachine) UpdateStatus(
	resourceName string,
	resource interface{},
	state ResourceState,
) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()
	disp := display.GetGlobalDisplay()

	if _, ok := dsm.Resources[resourceName]; !ok {
		dsm.Resources[resourceName] = StateMachineResource{
			Type:        reflect.TypeOf(resource),
			Resource:    resource,
			State:       state,
			LastUpdated: time.Now(),
		}
	}

	if dsm.Resources[resourceName].State > state {
		return
	}

	thisResource := dsm.Resources[resourceName]
	thisResource.State = state
	thisResource.LastUpdated = time.Now()
	dsm.Resources[resourceName] = thisResource

	stub := strings.Split(resourceName, "-")[0]

	isLocation := false
	deployment := GetGlobalDeployment()
	for _, machine := range deployment.Machines {
		if machine.Location == stub {
			isLocation = true
			disp.UpdateStatus(&models.Status{
				ID:     machine.Name,
				Status: state.String(),
			})
		}
	}

	// It was a location, so we've already applied it to all machines. Return.
	if isLocation {
		return
	}

	for _, machine := range deployment.Machines {
		if machine.ID == stub {
			disp.UpdateStatus(&models.Status{
				ID:     machine.Name,
				Status: state.String(),
			})
		}
	}
}

func (dsm *AzureStateMachine) GetStatus(key string) (ResourceState, bool) {
	state, ok := dsm.Resources[key]
	if !ok {
		return StateNotStarted, false
	}
	return state.State, true
}

func (dsm *AzureStateMachine) GetTotalResourcesCount() int {
	return len(dsm.Resources)
}

func (dsm *AzureStateMachine) GetAllResources() map[string]StateMachineResource {
	return dsm.Resources
}
