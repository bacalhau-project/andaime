package internal_azure

import (
	"sort"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"gopkg.in/yaml.v2"
)

type AzureData struct {
	Locations map[string][]string `yaml:"locations"`
}

func getSortedAzureData() ([]byte, error) {
	data, err := GetAzureData()
	if err != nil {
		return nil, err
	}

	var azureData AzureData
	err = yaml.Unmarshal(data, &azureData)
	if err != nil {
		return nil, err
	}

	// Sort locations
	sortedLocations := make([]string, 0, len(azureData.Locations))
	for location := range azureData.Locations {
		sortedLocations = append(sortedLocations, location)
	}
	sort.Strings(sortedLocations)

	// Sort VM sizes for each location
	sortedData := AzureData{
		Locations: make(map[string][]string),
	}
	for _, location := range sortedLocations {
		vmSizes := azureData.Locations[location]
		sort.Strings(vmSizes)
		sortedData.Locations[location] = vmSizes
	}

	return yaml.Marshal(sortedData)
}

func IsValidAzureRegion(region string) bool {
	l := logger.Get()
	data, err := getSortedAzureData()
	if err != nil {
		l.Warnf("Failed to get sorted Azure data: %v", err)
		return false
	}

	var azureData AzureData
	err = yaml.Unmarshal(data, &azureData)
	if err != nil {
		l.Warnf("Failed to unmarshal Azure data: %v", err)
		return false
	}

	_, exists := azureData.Locations[strings.ToLower(region)]
	return exists
}

func IsValidAzureVMSize(location, vmSize string) bool {
	l := logger.Get()
	data, err := getSortedAzureData()
	if err != nil {
		l.Warnf("Failed to get sorted Azure data: %v", err)
		return false
	}

	var azureData AzureData
	err = yaml.Unmarshal(data, &azureData)
	if err != nil {
		l.Warnf("Failed to unmarshal Azure data: %v", err)
		return false
	}

	vmSizes, exists := azureData.Locations[location]
	if !exists {
		l.Warnf("Location not found: %s", location)
		return false
	}

	for _, size := range vmSizes {
		if size == vmSize {
			return true
		}
	}

	l.Warnf("Invalid VM size for location: %s, vmSize: %s", location, vmSize)
	return false
}
