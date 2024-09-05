package internal_gcp

import (
	"slices"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"gopkg.in/yaml.v2"
)

type GCPData struct {
	Locations map[string][]string `yaml:"locations"`
}

func IsValidGCPLocation(location string) bool {
	l := logger.Get()
	gcpDataRaw, err := GetGCPData()
	if err != nil {
		return false
	}

	var gcpData GCPData
	err = yaml.Unmarshal(gcpDataRaw, &gcpData)
	if err != nil {
		l.Warnf("Failed to unmarshal GCP data: %v", err)
		return false
	}

	_, exists := gcpData.Locations[strings.ToLower(location)]
	return exists
}

func IsValidGCPMachineType(location, machineType string) bool {
	l := logger.Get()
	gcpData, err := GetGCPData()
	if err != nil {
		return false
	}
	var validLocationsAndMachines GCPData
	err = yaml.Unmarshal(gcpData, &validLocationsAndMachines)
	if err != nil {
		l.Warnf("Failed to unmarshal GCP data: %v", err)
		return false
	}
	validMachineTypes, ok := validLocationsAndMachines.Locations[strings.ToLower(location)]
	if !ok {
		return false
	}
	return slices.Contains(validMachineTypes, machineType)
}
