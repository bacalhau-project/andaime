package models

import (
	"fmt"
	"strings"
	"time"
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
	SSH             string
	Docker          string
	Bacalhau        string
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
	}
}

func NewDisplayStatus(
	resourceID string,
	resourceType AzureResourceTypes,
	state AzureResourceState,
) *DisplayStatus {
	return &DisplayStatus{
		ID:   resourceID,
		Name: resourceID,
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
	DisplayEmojiSuccess    = "✔" // "✅"
	DisplayEmojiWaiting    = "⟳" // "⏳"
	DisplayEmojiFailed     = "✘" // "❌"
	DisplayEmojiQuestion   = "?" // "❓"
	DisplayEmojiNotStarted = "┅" // "⬛️"

	DisplayEmojiOrchestratorNode = "⏼" // "🌕"
	DisplayEmojiWorkerNode       = " " // "⚫️"

	DisplayEmojiOrchestrator = "O" // "🤖"
	DisplayEmojiSSH          = "S" // "🔑"
	DisplayEmojiDocker       = "D" // "🐳"
	DisplayEmojiBacalhau     = "B" // "🐟"
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
	machines []Machine,
) ([]DisplayStatus, error) {
	resourceName := resourceMap["name"].(string)
	resourceType := resourceMap["type"].(string)
	resourceState := resourceMap["provisioningState"].(string)

	var statuses []DisplayStatus

	if location, ok := isLocation(resourceName); ok {
		machinesNames, err := GetMachinesInLocation(location, machines)
		if err != nil {
			return nil, err
		}
		for _, machineName := range machinesNames {
			machine, err := GetMachineByName(machineName, machines)
			if err != nil {
				return nil, err
			}
			status := createStatus(machine.Name, resourceName, resourceType, resourceState)
			statuses = append(statuses, status)
		}
	} else if machineName, ok := isMachine(resourceName); ok {
		status := createStatus(machineName, resourceName, resourceType, resourceState)
		statuses = append(statuses, status)
	} else {
		return nil, fmt.Errorf("unknown resource ID format: %s", resourceName)
	}

	return statuses, nil
}

func isLocation(id string) (string, bool) {
	if strings.HasSuffix(id, "-nsg") || strings.HasSuffix(id, "-vnet") {
		return strings.Split(id, "-")[0], true
	}
	return "", false
}

func isMachine(id string) (string, bool) {
	if strings.Contains(id, "-vm") || strings.Contains(id, "-vm-") {
		return fmt.Sprintf("%s-vm", strings.Split(id, "-")[0]), true
	}
	return "", false
}

func GetMachinesInLocation(resourceName string, machines []Machine) ([]string, error) {
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

func GetMachineByName(name string, machines []Machine) (Machine, error) {
	for _, machine := range machines {
		if machine.Name == name {
			return machine, nil
		}
	}
	return Machine{}, fmt.Errorf("machine not found: %s", name)
}

func createStatus(machineID, resourceID, resourceType, state string) DisplayStatus {
	azureResourceType := GetAzureResourceType(resourceType)
	stateType := ConvertFromStringToAzureResourceState(state)

	return DisplayStatus{
		ID:   machineID,
		Type: azureResourceType,
		StatusMessage: CreateStateMessage(
			azureResourceType,
			stateType,
			resourceID,
		),
	}
}
