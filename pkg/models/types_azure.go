package models

import (
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

var RequiredAzureResources = []ResourceType{
	AzureResourceTypeVNET,
	AzureResourceTypeNIC,
	AzureResourceTypeNSG,
	AzureResourceTypeIP,
	AzureResourceTypeDISK,
	AzureResourceTypeVM,
}

var AzureSkippedResourceTypes = []string{
	"Microsoft.Compute/virtualMachines/extensions",
}

func (a *ResourceType) GetResourceLowerString() string {
	return strings.ToLower(a.ResourceString)
}

var AzureResourceTypeNIC = ResourceType{
	ResourceString:    "Microsoft.Network/networkInterfaces",
	ShortResourceName: "NIC ",
}

var AzureResourceTypeVNET = ResourceType{
	ResourceString:    "Microsoft.Network/virtualNetworks",
	ShortResourceName: "VNET",
}

var AzureResourceTypeSNET = ResourceType{
	ResourceString:    "Microsoft.Network/subnets",
	ShortResourceName: "SNET",
}

var AzureResourceTypeNSG = ResourceType{
	ResourceString:    "Microsoft.Network/networkSecurityGroups",
	ShortResourceName: "NSG ",
}

var AzureResourceTypeVM = ResourceType{
	ResourceString:    "Microsoft.Compute/virtualMachines",
	ShortResourceName: "VM  ",
}

var AzureResourceTypeDISK = ResourceType{
	ResourceString:    "Microsoft.Compute/disks",
	ShortResourceName: "DISK",
}

var AzureResourceTypeIP = ResourceType{
	ResourceString:    "Microsoft.Network/publicIPAddresses",
	ShortResourceName: "IP  ",
}

var AzureResourceTypeGuestAttestation = ResourceType{
	ResourceString:    "Microsoft.Compute/virtualMachines/extensions/GuestAttestation",
	ShortResourceName: "GATT",
}

func GetAzureResourceType(resource string) ResourceType {
	for _, r := range GetAllAzureResources() {
		if strings.EqualFold(r.ResourceString, resource) {
			return r
		}
	}
	return ResourceType{}
}

func GetAllAzureResources() []ResourceType {
	return []ResourceType{
		AzureResourceTypeNIC,
		AzureResourceTypeVNET,
		AzureResourceTypeSNET,
		AzureResourceTypeNSG,
		AzureResourceTypeVM,
		AzureResourceTypeDISK,
		AzureResourceTypeIP,
		AzureResourceTypeGuestAttestation,
	}
}

func IsValidAzureResource(resource string) bool {
	return GetAzureResourceType(resource).ResourceString != ""
}

func ConvertFromAzureStringToResourceState(s string) MachineResourceState {
	l := logger.Get()
	switch s {
	case "Not Started":
		return ResourceStateNotStarted
	case "Pending":
		return ResourceStatePending
	case "Creating":
		return ResourceStatePending
	case "Failed":
		return ResourceStateFailed
	case "Succeeded":
		return ResourceStateSucceeded
	case "Updating":
		return ResourceStateSucceeded
	case "Running":
		return ResourceStateSucceeded
	default:
		l.Debugf("Unknown Azure Resource State: %s", s)
		return ResourceStateUnknown
	}
}
