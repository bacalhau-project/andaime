package models

import (
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

var RequiredGCPResources = []ResourceTypes{
	GCPResourceTypeProject,
	GCPResourceTypeVPC,
	GCPResourceTypeFirewall,
	GCPResourceTypeInstance,
	GCPResourceTypeDisk,
}

var SkippedResourceTypes = []string{
	"compute.v1.instanceGroupManager",
}

var GCPResourceTypeProject = ResourceTypes{
	ResourceString:    "cloudresourcemanager.googleapis.com/Project",
	ShortResourceName: "PRJ ",
}

var GCPResourceTypeVPC = ResourceTypes{
	ResourceString:    "compute.googleapis.com/Network",
	ShortResourceName: "VPC ",
}

var GCPResourceTypeFirewall = ResourceTypes{
	ResourceString:    "compute.googleapis.com/Firewall",
	ShortResourceName: "FW  ",
}

var GCPResourceTypeInstance = ResourceTypes{
	ResourceString:    "compute.googleapis.com/Instance",
	ShortResourceName: "VM  ",
}

var GCPResourceTypeDisk = ResourceTypes{
	ResourceString:    "compute.googleapis.com/Disk",
	ShortResourceName: "DISK",
}

func GetGCPResourceType(resource string) ResourceTypes {
	for _, r := range GetAllGCPResources() {
		if strings.EqualFold(r.ResourceString, resource) {
			return r
		}
	}
	return ResourceTypes{}
}

func GetAllGCPResources() []ResourceTypes {
	return []ResourceTypes{
		GCPResourceTypeProject,
		GCPResourceTypeVPC,
		GCPResourceTypeFirewall,
		GCPResourceTypeInstance,
		GCPResourceTypeDisk,
	}
}

func IsValidGCPResource(resource string) bool {
	return GetGCPResourceType(resource).ResourceString != ""
}

type GCPResourceState int

const (
	GCPResourceStateUnknown GCPResourceState = iota
	GCPResourceStateProvisioning
	GCPResourceStateRunning
	GCPResourceStateStopping
	GCPResourceStateRepairing
	GCPResourceStateTerminated
	GCPResourceStateSuspended
)

func ConvertFromStringToGCPResourceState(s string) GCPResourceState {
	l := logger.Get()
	switch s {
	case "PROVISIONING":
		return GCPResourceStateProvisioning
	case "RUNNING":
		return GCPResourceStateRunning
	case "STOPPING":
		return GCPResourceStateStopping
	case "REPAIRING":
		return GCPResourceStateRepairing
	case "TERMINATED":
		return GCPResourceStateTerminated
	case "SUSPENDED":
		return GCPResourceStateSuspended
	default:
		l.Debugf("Unknown GCP Resource State: %s", s)
		return GCPResourceStateUnknown
	}
}

const (
	MachineResourceTypeComputeInstance = "compute_instance"
)
