package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

type ProviderAbbreviation string

const (
	ProviderAbbreviationAzure   ProviderAbbreviation = "AZU"
	ProviderAbbreviationAWS     ProviderAbbreviation = "AWS"
	ProviderAbbreviationGCP     ProviderAbbreviation = "GCP"
	ProviderAbbreviationVirtual ProviderAbbreviation = "VIR"
	ProviderAbbreviationUnknown ProviderAbbreviation = "UNK"
)

type DisplayStatus struct {
	ID              string
	Type            AzureResourceTypes
	Location        string
	StatusMessage   string
	DetailedStatus  string
	ElapsedTime     time.Duration
	StartTime       time.Time
	InstanceID      string
	PublicIP        string
	PrivateIP       string
	HighlightCycles int
	Name            string
	Progress        int
	Orchestrator    bool
	SSH             ServiceState
	Docker          ServiceState
	CorePackages    ServiceState
	Bacalhau        ServiceState
}

func NewDisplayStatusWithText(
	resourceID string,
	resourceType AzureResourceTypes,
	state AzureResourceState,
	text string,
) *DisplayStatus {
	return &DisplayStatus{
		ID:   resourceID,
		Name: resourceID,
		Type: resourceType,
		StatusMessage: CreateStateMessageWithText(
			resourceType,
			state,
			resourceID,
			text,
		),
		SSH:      ServiceStateUnknown,
		Docker:   ServiceStateUnknown,
		Bacalhau: ServiceStateUnknown,
	}
}

// NewDisplayVMStatus creates a new DisplayStatus for a VM
// - machineName is the name of the machine (the start of the row - should be unique, something like ABCDEF-vm)
// - resourceType is the type of the resource (e.g. AzureResourceTypeNIC)
// - state is the state of the resource (e.g. AzureResourceStateSucceeded)
func NewDisplayVMStatus(
	machineName string,
	state AzureResourceState,
) *DisplayStatus {
	return NewDisplayStatus(machineName, machineName, AzureResourceTypeVM, state)
}

// NewDisplayStatus creates a new DisplayStatus
// - machineName is the name of the machine (the start of the row - should be unique, something like ABCDEF-vm)
// - resourceID is the name of the resource (the end of the row - should be unique, something like ABCDEF-vm-nic or centralus-vnet)
// - resourceType is the type of the resource (e.g. AzureResourceTypeNIC)
// - state is the state of the resource (e.g. AzureResourceStateSucceeded)
//
//nolint:lll
func NewDisplayStatus(
	machineName string,
	resourceID string,
	resourceType AzureResourceTypes,
	state AzureResourceState,
) *DisplayStatus {
	return &DisplayStatus{
		ID:   machineName,
		Name: machineName,
		Type: resourceType,
		StatusMessage: CreateStateMessage(
			resourceType,
			state,
			resourceID,
		),
	}
}

const (
	StatusCodeNotStarted StatusCode = "NotStarted"
	StatusCodeSucceeded  StatusCode = "Succeeded"
	StatusCodeFailed     StatusCode = "Failed"
	StatusCodeInProgress StatusCode = "InProgress"
	StatusCodeUnknown    StatusCode = "Unknown"
)

type TimeUpdateMsg struct{}

type AzureEvent struct {
	Type       string
	ResourceID string
	Message    string
}

const (
	DisplayTextSuccess    = "✔"
	DisplayTextFailed     = "✘"
	DisplayTextInProgress = "↻"
	DisplayTextWaiting    = "↻"
	DisplayTextCreating   = "⌃"
	DisplayTextUnknown    = "?"
	DisplayTextNotStarted = "┅"

	DisplayEmojiSuccess    = "✅"
	DisplayEmojiWaiting    = "⏳"
	DisplayEmojiCreating   = "⬆️"
	DisplayEmojiFailed     = "❌"
	DisplayEmojiQuestion   = "❓"
	DisplayEmojiNotStarted = "⬛️"

	DisplayEmojiOrchestratorNode = "🌕"
	DisplayEmojiWorkerNode       = "⚫️"

	DisplayTextOrchestratorNode = "⏼"
	DisplayTextWorkerNode       = " "

	DisplayEmojiOrchestrator = "🤖"
	DisplayEmojiSSH          = "🔑"
	DisplayEmojiDocker       = "🐳"
	DisplayEmojiBacalhau     = "🐟"

	DisplayTextOrchestrator = "O"
	DisplayTextSSH          = "S"
	DisplayTextDocker       = "D"
	DisplayTextBacalhau     = "B"
)

func CreateStateMessageWithText(
	resource AzureResourceTypes,
	resourceState AzureResourceState,
	resourceName string,
	text string,
) string {
	return CreateStateMessage(resource, resourceState, resourceName) + " " + text
}

func CreateStateMessage(
	resource AzureResourceTypes,
	resourceState AzureResourceState,
	resourceName string,
) string {
	l := logger.Get()
	stateEmoji := ""
	switch resourceState {
	case AzureResourceStateNotStarted:
		stateEmoji = DisplayEmojiNotStarted
	case AzureResourceStatePending:
		stateEmoji = DisplayEmojiWaiting
	case AzureResourceStateRunning:
		stateEmoji = DisplayEmojiSuccess
	case AzureResourceStateFailed:
		stateEmoji = DisplayEmojiFailed
	case AzureResourceStateSucceeded:
		stateEmoji = DisplayEmojiSuccess
	case AzureResourceStateUnknown:
		l.Debugf("Resource: %s", resource)
		l.Debugf("Resource Name: %s", resourceName)
		l.Debugf("Resource State: %d", resourceState)
		stateEmoji = DisplayEmojiQuestion
	}
	return fmt.Sprintf(
		"%s %s - %s",
		resource.ShortResourceName,
		stateEmoji,
		resourceName,
	)
}

func ConvertFromRawResourceToStatus(
	resourceMap map[string]interface{},
	deployment *Deployment,
) ([]DisplayStatus, error) {
	resourceName := resourceMap["name"].(string)
	resourceType := resourceMap["type"].(string)
	resourceState := resourceMap["provisioningState"].(string)

	var statuses []DisplayStatus

	if location := GetLocationFromResourceName(resourceName); location != "" {
		machinesNames, err := GetMachinesInLocation(location, deployment.Machines)
		if err != nil {
			return nil, err
		}
		for _, machineName := range machinesNames {
			if machineNeedsUpdating(
				deployment,
				machineName,
				resourceType,
				resourceState,
			) {
				status := createStatus(machineName, resourceName, resourceType, resourceState)
				statuses = append(statuses, status)
			}
		}
	} else if machineName := GetMachineNameFromResourceName(resourceName); machineName != "" {
		if machineNeedsUpdating(
			deployment,
			machineName,
			resourceType,
			resourceState,
		) {
			status := createStatus(machineName, resourceName, resourceType, resourceState)
			statuses = append(statuses, status)
		}
	} else {
		if !utils.CaseInsensitiveContains(SkippedResourceTypes, resourceType) {
			return nil, fmt.Errorf("unknown resource ID format: %s", resourceName)
		}
	}

	return statuses, nil
}

func GetLocationFromResourceName(id string) string {
	if strings.HasSuffix(id, "-nsg") || strings.HasSuffix(id, "-vnet") {
		return strings.Split(id, "-")[0]
	}
	return ""
}

// Tests to see if the resource name is a machine ID. Returns the machine ID if it is.
func GetMachineNameFromResourceName(id string) string {
	if strings.Contains(id, "-vm") || strings.Contains(id, "-vm-") {
		return fmt.Sprintf("%s-vm", strings.Split(id, "-")[0])
	}
	return ""
}

func machineNeedsUpdating(
	deployment *Deployment,
	machineName string,
	resourceType string,
	resourceState string,
) bool {
	// l := logger.Get()
	// l.Debugf(
	// 	"machineNeedsUpdating: %s, %s, %s",
	// 	deployment.Machines[machineIndex].Name,
	// 	resourceType,
	// 	resourceState,
	// )
	currentState := ConvertFromStringToAzureResourceState(resourceState)

	needsUpdate := 0
	if (deployment.Machines[machineName].GetResource(resourceType) == MachineResource{}) ||
		(deployment.Machines[machineName].GetResource(resourceType).ResourceState < currentState) {
		deployment.Machines[machineName].SetResource(resourceType, currentState)
		needsUpdate++
	}
	return needsUpdate > 0
}

func GetMachinesInLocation(resourceName string, machines map[string]*Machine) ([]string, error) {
	location := strings.Split(resourceName, "-")[0]

	if location == "" {
		return nil, fmt.Errorf("location is empty")
	}

	var machinesInLocation []string

	for _, machine := range machines {
		if machine.Location == location {
			machinesInLocation = append(machinesInLocation, machine.Name)
		}
	}

	return machinesInLocation, nil
}

// createStatus creates a new DisplayStatus
// - machineName is the name of the machine (the start of the row - should be unique, something like ABCDEF-vm)
// - resourceID is the name of the resource (the end of the row - should be unique, something like ABCDEF-vm-nic or centralus-vnet)
// - resourceType is the type of the resource (e.g. AzureResourceTypeNIC)
// - state is the state of the resource (e.g. AzureResourceStateSucceeded)
//
//nolint:lll
func createStatus(machineName, resourceID, resourceType, state string) DisplayStatus {
	azureResourceType := GetAzureResourceType(resourceType)
	stateType := ConvertFromStringToAzureResourceState(state)

	return *NewDisplayStatus(machineName, resourceID, azureResourceType, stateType)
}

func UpdateOnlyChangedStatus(
	status *DisplayStatus,
	newStatus *DisplayStatus,
) *DisplayStatus {
	if newStatus.StatusMessage != "" {
		status.StatusMessage = newStatus.StatusMessage
	}

	if newStatus.DetailedStatus != "" {
		status.DetailedStatus = newStatus.DetailedStatus
	}
	if newStatus.PublicIP != "" {
		status.PublicIP = newStatus.PublicIP
	}
	if newStatus.PrivateIP != "" {
		status.PrivateIP = newStatus.PrivateIP
	}
	if newStatus.InstanceID != "" {
		status.InstanceID = newStatus.InstanceID
	}
	if newStatus.Location != "" {
		status.Location = newStatus.Location
	}

	if status.StartTime.IsZero() {
		status.StartTime = time.Now()
	}

	status.ElapsedTime = newStatus.ElapsedTime

	return status
}
