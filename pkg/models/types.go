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
	resourceID := resourceMap["id"].(string)
	resourceType := resourceMap["type"].(string)
	resourceState := resourceMap["provisioningState"].(string)

	var statuses []DisplayStatus

	if isLocation(resourceID) {
		machines, err := GetMachinesInLocation(resourceID)
		if err != nil {
			return nil, err
		}
		for _, machine := range machines {
			status := createStatus(machine, resourceType, resourceState)
			statuses = append(statuses, status)
		}
	} else if isMachine(resourceID) {
		status := createStatus(resourceID, resourceType, resourceState)
		statuses = append(statuses, status)
	} else {
		return nil, fmt.Errorf("unknown resource ID format: %s", resourceID)
	}

	return statuses, nil
}

func GetMachinesInLocation(location string) ([]string, error) {
	// This is a mock implementation. In a real scenario, you'd query your infrastructure
	// to get the actual list of machines in the given location.
	allIDs := []string{
		"centralus-nsg", "centralus-vnet", "ddhtzx-vm-ip", "eastus-nsg", "eastus-vnet",
		"eastus2-nsg", "eastus2-vnet", "fj369e-vm", "fj369e-vm-ip", "fj369e-vm-nic",
		"fj369e-vm_OsDisk_1_b9cca35bff6d4cb1b476c720e0b4c2b5", "jv2m3q-vm", "jv2m3q-vm-ip",
		"jv2m3q-vm-nic", "jv2m3q-vm_OsDisk_1_426c3e7af41d410db984ebe67d0308b1", "zf8o9i-vm",
		"zf8o9i-vm-ip", "zf8o9i-vm-nic", "zf8o9i-vm_OsDisk_1_700ecbdcd8bd4dfcae6efeaebda28d51",
	}

	var machinesInLocation []string
	for _, id := range allIDs {
		if strings.HasPrefix(id, location) && isMachine(id) {
			machinesInLocation = append(machinesInLocation, id)
		}
	}

	return machinesInLocation, nil
}
func isLocation(id string) bool {
	return strings.HasSuffix(id, "-nsg") || strings.HasSuffix(id, "-vnet")
}

func isMachine(id string) bool {
	return strings.Contains(id, "-vm") || strings.Contains(id, "-vm-")
}

func createStatus(resourceID, resourceType, state string) Status {
	var resourceTypeEnum UpdateStatusResourceType
	var emoji string

	switch {
	case strings.Contains(resourceID, "-vm"):
		resourceTypeEnum = UpdateStatusResourceTypeVM
		emoji = DisplayEmojiWaiting
	case strings.Contains(resourceID, "-nsg"):
		resourceTypeEnum = UpdateStatusResourceTypeNSG
		emoji = DisplayEmojiWaiting
	case strings.Contains(resourceID, "-vnet"):
		resourceTypeEnum = UpdateStatusResourceTypeVNET
		emoji = DisplayEmojiWaiting
	default:
		resourceTypeEnum = UpdateStatusResourceTypeUNK
		emoji = DisplayEmojiQuestion
	}

	statusCode, _ := ConvertFromStringToResourceState(state)
	statusMessage := fmt.Sprintf("%s %s - %s of %s", resourceTypeEnum, emoji, statusCode, resourceID)

	return Status{
		ID:     resourceID,
		Type:   resourceTypeEnum,
		Status: statusMessage,
		State:  statusCode,
	}
}
