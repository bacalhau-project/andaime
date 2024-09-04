package internal_gcp

import (
	"slices"

	"gopkg.in/yaml.v2"
)

func IsValidGCPLocation(location string) bool {
	gcpData, err := GetGCPData()
	if err != nil {
		return false
	}

	validLocationsAndMachines := make(map[string][]string)
	err = yaml.Unmarshal(gcpData, &validLocationsAndMachines)
	if err != nil {
		return false
	}
	if _, ok := validLocationsAndMachines[location]; !ok {
		return false
	}
	return true
}

func IsValidGCPMachineType(location, machineType string) bool {
	gcpData, err := GetGCPData()
	if err != nil {
		return false
	}

	validLocationsAndMachines := make(map[string][]string)
	err = yaml.Unmarshal(gcpData, &validLocationsAndMachines)
	if err != nil {
		return false
	}
	validMachineTypes, ok := validLocationsAndMachines[location]
	if !ok {
		return false
	}
	return slices.Contains(validMachineTypes, machineType)
}
