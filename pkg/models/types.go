package models

import (
	"fmt"
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
}

const (
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
	switch StatusCode(state) {
	case StatusCodeSucceeded, StatusCodeFailed, StatusCodeInProgress:
		return StatusCode(state), nil
	default:
		return StatusCodeUnknown, fmt.Errorf("unknown state: %s", state)
	}
}

func ConvertFromRawResourceToStatus(resourceMap map[string]interface{}) ([]DisplayStatus, error) {
	state, err := ConvertFromStringToResourceState(resourceMap["provisioningState"].(string))
	if err != nil {
		return nil, err
	}

	resourceID := resourceMap["id"].(string)
	resourceType := resourceMap["type"].(string)
	resourceStatus := resourceMap["status"].(string)

	var statuses []DisplayStatus

	// If the ID is a location (such as "centralus-nsg" or "centralus-vnet"), we will create a status
	// update for every machine in the location.

	// Else if the ID is a machine (such as "UNIQID-vm-ip" or "UNIQID-vm"), we will create a status

	return statuses, nil
}

// GetMachinesInLocation is a placeholder function
// You'll need to implement this based on your actual data structure
func GetMachinesInLocation(location string) ([]Machine, error) {
	// Implementation depends on how you store and retrieve machine information
	return nil, fmt.Errorf("GetMachinesInLocation not implemented")
}
