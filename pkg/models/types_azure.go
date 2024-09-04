package models

import (
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

var RequiredAzureResources = []ResourceTypes{
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

func (a *ResourceTypes) GetResourceLowerString() string {
	return strings.ToLower(a.ResourceString)
}

var AzureResourceTypeNIC = ResourceTypes{
	ResourceString:    "Microsoft.Network/networkInterfaces",
	ShortResourceName: "NIC ",
}

var AzureResourceTypeVNET = ResourceTypes{
	ResourceString:    "Microsoft.Network/virtualNetworks",
	ShortResourceName: "VNET",
}

var AzureResourceTypeSNET = ResourceTypes{
	ResourceString:    "Microsoft.Network/subnets",
	ShortResourceName: "SNET",
}

var AzureResourceTypeNSG = ResourceTypes{
	ResourceString:    "Microsoft.Network/networkSecurityGroups",
	ShortResourceName: "NSG ",
}

var AzureResourceTypeVM = ResourceTypes{
	ResourceString:    "Microsoft.Compute/virtualMachines",
	ShortResourceName: "VM  ",
}

var AzureResourceTypeDISK = ResourceTypes{
	ResourceString:    "Microsoft.Compute/disks",
	ShortResourceName: "DISK",
}

var AzureResourceTypeIP = ResourceTypes{
	ResourceString:    "Microsoft.Network/publicIPAddresses",
	ShortResourceName: "IP  ",
}

var AzureResourceTypeGuestAttestation = ResourceTypes{
	ResourceString:    "Microsoft.Compute/virtualMachines/extensions/GuestAttestation",
	ShortResourceName: "GATT",
}

func GetAzureResourceType(resource string) ResourceTypes {
	for _, r := range GetAllAzureResources() {
		if strings.EqualFold(r.ResourceString, resource) {
			return r
		}
	}
	return ResourceTypes{}
}

func GetAllAzureResources() []ResourceTypes {
	return []ResourceTypes{
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

func ConvertFromAzureStringToResourceState(s string) ResourceState {
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
