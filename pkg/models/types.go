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

type Status struct {
	ID              string
	Type            UpdateStatusResourceType
	Location        string
	Status          string
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
	State           StatusCode
}

type TimeUpdateMsg struct{}

// Remove these duplicate declarations as they are already defined in deployment.go

type AzureEvent struct {
	Type       string
	ResourceID string
	Message    string
}

const (
	DisplayPrefixRG   = "RG  "
	DisplayPrefixVNET = "VNET"
	DisplayPrefixSNET = "SNET"
	DisplayPrefixNSG  = "NSG "
	DisplayPrefixVM   = "VM  "
	DisplayPrefixVMEX = "VMEX"
	DisplayPrefixDISK = "DISK"
	DisplayPrefixIP   = "IP  "
	DisplayPrefixPBIP = "PBIP"
	DisplayPrefixPVIP = "PVIP"
	DisplayPrefixNIC  = "NIC "
	DisplayPrefixUNK  = "UNK "

	DisplayEmojiSuccess    = "✔" // "✅"
	DisplayEmojiWaiting    = "⟳" // "⏳"
	DisplayEmojiFailed     = "✘" // "❌"
	DisplayEmojiQuestion   = "?" // "❓"
	DisplayEmojiNotStarted = "┅" // "⬛️"

	DisplayEmojiOrchestratorNode = "⏼" // "🌕"
	DisplayEmojiWorkerNode       = " " // "⚫️"

	DisplayEmojiOrchestrator = "🤖"
	DisplayEmojiSSH          = "🔑"
	DisplayEmojiDocker       = "🐳"
	DisplayEmojiBacalhau     = "🐟"
)

type UpdateStatusResourceType string

const (
	UpdateStatusResourceTypeVM   UpdateStatusResourceType = "VM"
	UpdateStatusResourceTypeVMEX UpdateStatusResourceType = "VMEX"
	UpdateStatusResourceTypePBIP UpdateStatusResourceType = "PBIP"
	UpdateStatusResourceTypePVIP UpdateStatusResourceType = "PVIP"
	UpdateStatusResourceTypeNIC  UpdateStatusResourceType = "NIC"
	UpdateStatusResourceTypeNSG  UpdateStatusResourceType = "NSG"
	UpdateStatusResourceTypeVNET UpdateStatusResourceType = "VNET"
	UpdateStatusResourceTypeSNET UpdateStatusResourceType = "SNET"
	UpdateStatusResourceTypeDISK UpdateStatusResourceType = "DISK"
	UpdateStatusResourceTypeIP   UpdateStatusResourceType = "IP"
	UpdateStatusResourceTypeUNK  UpdateStatusResourceType = "UNK"
)

func CreateStateMessage(
	resourceName UpdateStatusResourceType,
	stateString StatusCode,
	machineName string,
) string {
	return fmt.Sprintf("%s %s - %s", resourceName, stateString, machineName)
}

func ConvertFromStringToResourceState(state string) (StatusCode, error) {
	// This is a placeholder implementation. Adjust according to your actual status codes.
	switch state {
	case "Succeeded":
		return StatusCodeSucceeded, nil
	case "Failed":
		return StatusCodeFailed, nil
	case "InProgress":
		return StatusCodeInProgress, nil
	default:
		return StatusCodeUnknown, fmt.Errorf("unknown state: %s", state)
	}
}

func ConvertFromRawResourceToStatus(resourceMap map[string]interface{}) ([]Status, error) {
	state, err := ConvertFromStringToResourceState(resourceMap["provisioningState"].(string))
	if err != nil {
		return nil, err
	}

	resourceID := resourceMap["id"].(string)
	resourceType := resourceMap["type"].(string)
	resourceStatus := resourceMap["status"].(string)

	var statuses []Status

	if strings.Contains(resourceID, "-vm") {
		// This is a VM resource
		status := Status{
			ID:     resourceID,
			Type:   UpdateStatusResourceTypeVM,
			Status: resourceStatus,
			State:  state,
		}
		statuses = append(statuses, status)
	} else if strings.Contains(resourceID, "-nsg") {
		// This is an NSG resource, create a status for each machine in the location
		location := strings.Split(resourceID, "-")[0]
		// Assuming we have a function to get all machines in a location
		machines, err := GetMachinesInLocation(location)
		if err != nil {
			return nil, err
		}
		for _, machine := range machines {
			status := Status{
				ID:     machine.ID,
				Type:   UpdateStatusResourceTypeNSG,
				Status: resourceStatus,
				State:  state,
			}
			statuses = append(statuses, status)
		}
	} else {
		// Unknown resource type
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	return statuses, nil
}

// GetMachinesInLocation is a placeholder function
// You'll need to implement this based on your actual data structure
func GetMachinesInLocation(location string) ([]Machine, error) {
	// Implementation depends on how you store and retrieve machine information
	return nil, fmt.Errorf("GetMachinesInLocation not implemented")
}
