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
	PrivateIP       string
	HighlightCycles int
}

type AzureEvent struct {
	Type       string
	ResourceID string
	Message    string
}
