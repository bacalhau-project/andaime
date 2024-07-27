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
	Region          string
	Zone            string
	Status          string
	DetailedStatus  string
	ElapsedTime     time.Duration
	InstanceID      string
	PublicIP        string
	PrivateIP       string
	HighlightCycles int
}
