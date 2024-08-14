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
