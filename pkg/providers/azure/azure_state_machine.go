package azure

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
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
	Name        string
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

func CreateStateMessage(resourceType string, state ResourceState, resourceName string) string {
	prefix := ""
	switch resourceType {
	case "VM":
		prefix = DisplayPrefixVM
	case "PBIP":
		prefix = DisplayPrefixPBIP
	case "PVIP":
		prefix = DisplayPrefixPVIP
	case "NIC":
		prefix = DisplayPrefixNIC
	case "NSG":
		prefix = DisplayPrefixNSG
	case "VNET":
		prefix = DisplayPrefixVNET
	case "SNET":
		prefix = DisplayPrefixSNET
	case "DISK":
		prefix = DisplayPrefixDISK
	}

	emoji := ""
	switch state {
	case StateSucceeded:
		emoji = DisplayEmojiSuccess
	case StateProvisioning:
		emoji = DisplayEmojiWaiting
	case StateFailed:
		emoji = DisplayEmojiFailed
	}

	return fmt.Sprintf("%s %s = %s", prefix, emoji, resourceName)
}

func NewDeploymentStateMachine(deployment *models.Deployment) *AzureStateMachine {
	return &AzureStateMachine{
		Resources: make(map[string]StateMachineResource),
	}
}

func (dsm *AzureStateMachine) UpdateStatus(
	resourceName string,
	resourceType string,
	resource interface{},
	state ResourceState,
) {
	l := logger.Get()
	dsm.mu.Lock()
	defer dsm.mu.Unlock()
	disp := display.GetGlobalDisplay()

	if _, ok := dsm.Resources[resourceName]; !ok {
		dsm.Resources[resourceName] = StateMachineResource{
			Resource:    resource,
			State:       state,
			LastUpdated: time.Now(),
		}
	}

	if dsm.Resources[resourceName].State >= state {
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
			l.Debugf("Updating status for location %s to %s", machine.Name, state.String())
			disp.UpdateStatus(&models.Status{
				ID:     machine.Name,
				Status: CreateStateMessage(resourceType, state, resourceName),
			})
		}
	}

	// It was a location, so we've already applied it to all machines. Return.
	if isLocation {
		return
	}

	for _, machine := range deployment.Machines {
		if machine.ID == stub {
			l.Debugf("Updating status for machine %s to %s", machine.Name, state.String())
			l.Debugf("Full Object: %s", resource)
			disp.UpdateStatus(&models.Status{
				ID:     machine.Name,
				Status: CreateStateMessage(resourceType, state, resourceName),
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
