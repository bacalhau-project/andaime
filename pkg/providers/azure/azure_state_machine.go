package azure

import (
	"errors"
	"fmt"
	"slices"
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
	skipTypes            = []string{"microsoft.network/networkwatchers"}
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

func CreateStateMessage(
	resourceType models.UpdateStatusResourceType,
	state ResourceState,
	resourceName string,
) string {
	l := logger.Get()

	prefix := models.DisplayPrefixUNK
	switch resourceType {
	case models.UpdateStatusResourceTypeVM:
		prefix = models.DisplayPrefixVM
	case models.UpdateStatusResourceTypeIP:
		prefix = models.DisplayPrefixIP
	case models.UpdateStatusResourceTypePBIP:
		prefix = models.DisplayPrefixPBIP
	case models.UpdateStatusResourceTypePVIP:
		prefix = models.DisplayPrefixPVIP
	case models.UpdateStatusResourceTypeNIC:
		prefix = models.DisplayPrefixNIC
	case models.UpdateStatusResourceTypeNSG:
		prefix = models.DisplayPrefixNSG
	case models.UpdateStatusResourceTypeVNET:
		prefix = models.DisplayPrefixVNET
	case models.UpdateStatusResourceTypeSNET:
		prefix = models.DisplayPrefixSNET
	case models.UpdateStatusResourceTypeDISK:
		prefix = models.DisplayPrefixDISK
	default:
		l.Errorf("Unknown resource type: %s", resourceType)
	}

	emoji := models.DisplayEmojiQuestion
	switch state {
	case StateSucceeded:
		emoji = models.DisplayEmojiSuccess
	case StateProvisioning:
		emoji = models.DisplayEmojiWaiting
	case StateFailed:
		emoji = models.DisplayEmojiFailed
	default:
		l.Errorf("Unknown state: %s", state)
	}

	// Special case for resource name for disk - kl41d9-vm_disk1_effcdaed5bf14cbebb82bf65f2958ac6
	resourceName = strings.ReplaceAll(resourceName, "_", "-")

	return fmt.Sprintf("%s %s = %s", prefix, emoji, resourceName)
}

func NewDeploymentStateMachine(deployment *models.Deployment) *AzureStateMachine {
	return &AzureStateMachine{
		Resources: make(map[string]StateMachineResource),
	}
}

func CastResourceTypeForUpdateStatus(resourceType string) (models.UpdateStatusResourceType, error) {
	l := logger.Get()
	var resourceTypeForUpdateStatus models.UpdateStatusResourceType
	switch resourceType {
	case "microsoft.compute/virtualmachines":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeVM
	case "microsoft.network/virtualnetworks":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeVNET
	case "microsoft.network/virtualnetworks/subnets":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeSNET
	case "microsoft.compute/disks":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeDISK
	case "microsoft.network/publicipaddresses":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeIP
	case "microsoft.network/networkinterfaces":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeNIC
	case "microsoft.network/networksecuritygroups":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeNSG
	case "microsoft.compute/virtualmachines/extensions":
		resourceTypeForUpdateStatus = models.UpdateStatusResourceTypeVMEX
	default:
		l.Errorf("Unknown resource type: %s", resourceType)
		return models.UpdateStatusResourceTypeUNK, errors.New("unknown resource type")
	}

	return resourceTypeForUpdateStatus, nil
}

func (dsm *AzureStateMachine) UpdateStatus(
	resourceName string,
	resourceType models.UpdateStatusResourceType,
	resource interface{},
	state ResourceState,
) {
	l := logger.Get()

	if slices.Contains(skipTypes, string(resourceType)) {
		l.Debugf(
			"Skipping status update for %s %s",
			resourceType,
			resourceName,
		)
		return
	}

	dsm.mu.Lock()
	defer dsm.mu.Unlock()
	disp := display.GetGlobalDisplay()

	if _, ok := dsm.Resources[resourceName]; !ok {
		dsm.Resources[resourceName] = StateMachineResource{
			Resource:    resource,
			State:       -1,
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
