package models

import (
	"time"
)

type ResourceType string

const (
	ResourceTypeAzure ResourceType = "azure"
)

type Status struct {
	ID              string
	Type            string
	Location        string
	Status          string
	DetailedStatus  string
	ElapsedTime     time.Duration
	StartTime       time.Time
	InstanceID      string
	PublicIP        string
}

type Deployment struct {
	ProjectID             string
	UniqueID              string
	ResourceGroupName     string
	ResourceGroupLocation string
	Machines              []*Machine
	OrchestratorNode      *Machine
	NetworkSecurityGroups map[string]*armnetwork.SecurityGroup
	Subnets               map[string][]*armnetwork.Subnet
	SSHPublicKeyPath      string
	SSHPrivateKeyPath     string
	SSHPublicKeyData      []byte
	AllowedPorts          []int
	Tags                  map[string]*string
	StartTime             time.Time
	PrivateIP       string
	HighlightCycles int
}
