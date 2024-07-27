package models

import (
	"time"
)

type ResourceType string

const (
	ResourceTypeAzure ResourceType = "azure"
)

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
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
	PrivateIP       string
	HighlightCycles int
	PrivateIP       string
	HighlightCycles int
}
