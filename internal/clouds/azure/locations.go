package internal_azure

import (
	"slices"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"gopkg.in/yaml.v2"
)

func IsValidAzureLocation(location string) bool {
	l := logger.Get()
	data, err := GetAzureData()
	if err != nil {
		l.Warnf("Failed to get Azure data: %v", err)
		return false
	}

	var validLocations map[string]interface{}
	err = yaml.Unmarshal(data, &validLocations)
	if err != nil {
		l.Warnf("Failed to unmarshal Azure data: %v", err)
		return false
	}

	for validLocation := range validLocations {
		if strings.EqualFold(location, validLocation) {
			return true
		}
	}

	l.Warnf("Invalid Azure location: %s", location)
	return false
}

func IsValidAzureVMSize(location, vmSize string) bool {
	l := logger.Get()

	if !IsValidAzureLocation(location) {
		return false
	}

	data, err := GetAzureData()
	if err != nil {
		l.Warnf("Failed to get Azure data: %v", err)
		return false
	}

	validLocationAndVMType := make(map[string][]string)
	err = yaml.Unmarshal(data, &validLocationAndVMType)
	if err != nil {
		l.Warnf("Could not load Azure valid location and machines: %v", err)
		return false
	}

	locMachineTypes, ok := validLocationAndVMType[location]
	if !ok {
		l.Warnf("No VM sizes found for location: %s", location)
		return false
	}

	if !slices.Contains(locMachineTypes, vmSize) {
		l.Warnf("Invalid VM size for location: %s, vmSize: %s", location, vmSize)
		return false
	}

	return true
}
