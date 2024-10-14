package models

import (
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

var RequiredAWSResources = []ResourceType{
	AWSResourceTypeVPC,
	AWSResourceTypeSubnet,
	AWSResourceTypeSecurityGroup,
	AWSResourceTypeInstance,
	AWSResourceTypeVolume,
}

var AWSResourceTypeVPC = ResourceType{
	ResourceString:    "aws:ec2:vpc",
	ShortResourceName: "VPC ",
}

var AWSResourceTypeSubnet = ResourceType{
	ResourceString:    "aws:ec2:subnet",
	ShortResourceName: "SNET",
}

var AWSResourceTypeSecurityGroup = ResourceType{
	ResourceString:    "aws:ec2:security-group",
	ShortResourceName: "SG  ",
}

var AWSResourceTypeInstance = ResourceType{
	ResourceString:    "aws:ec2:instance",
	ShortResourceName: "EC2 ",
}

var AWSResourceTypeVolume = ResourceType{
	ResourceString:    "aws:ec2:volume",
	ShortResourceName: "VOL ",
}

func GetAWSResourceType(resource string) ResourceType {
	for _, r := range GetAllAWSResources() {
		if strings.EqualFold(r.ResourceString, resource) {
			return r
		}
	}
	return ResourceType{}
}

func GetAllAWSResources() []ResourceType {
	return []ResourceType{
		AWSResourceTypeVPC,
		AWSResourceTypeSubnet,
		AWSResourceTypeSecurityGroup,
		AWSResourceTypeInstance,
		AWSResourceTypeVolume,
	}
}

func IsValidAWSResource(resource string) bool {
	return GetAWSResourceType(resource).ResourceString != ""
}

type AWSResourceState int

const (
	AWSResourceStateUnknown AWSResourceState = iota
	AWSResourceStatePending
	AWSResourceStateRunning
	AWSResourceStateShuttingDown
	AWSResourceStateTerminated
	AWSResourceStateStopping
	AWSResourceStateStopped
)

func ConvertFromStringToAWSResourceState(s string) AWSResourceState {
	l := logger.Get()
	switch s {
	case "pending":
		return AWSResourceStatePending
	case "running":
		return AWSResourceStateRunning
	case "shutting-down":
		return AWSResourceStateShuttingDown
	case "terminated":
		return AWSResourceStateTerminated
	case "stopping":
		return AWSResourceStateStopping
	case "stopped":
		return AWSResourceStateStopped
	default:
		l.Debugf("Unknown AWS Resource State: %s", s)
		return AWSResourceStateUnknown
	}
}

const (
	MachineResourceTypeEC2Instance = "ec2_instance"
)
