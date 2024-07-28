package internal

import (
	"embed"
	"strings"
)

//go:embed machine_types.yaml
var machineTypes embed.FS

//go:embed locations.yaml
var locations embed.FS

//go:embed bicep/vm.json
var vmBicep embed.FS

func GetMachineTypes() ([]string, error) {
	data, err := machineTypes.ReadFile("machine_types.yaml")
	if err != nil {
		return nil, err
	}
	// Convert to string slice
	dataString := strings.Split(string(data), "\n")

	return dataString, nil
}

func GetLocations() ([]string, error) {
	data, err := locations.ReadFile("locations.yaml")
	if err != nil {
		return nil, err
	}
	// Convert to string slice
	dataString := strings.Split(string(data), "\n")

	return dataString, nil
}

func GetVMBicep() ([]byte, error) {
	data, err := vmBicep.ReadFile("bicep/vm.json")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func IsValidLocation(location string) bool {
	// This is a placeholder. In a real scenario, you would check against a list of valid Azure locations.
	validLocations, err := GetLocations()
	if err != nil {
		return false
	}
	if location == "" {
		return false
	}

	for _, validLocation := range validLocations {
		if location == validLocation {
			return true
		}
	}
	return false
}

func IsValidMachineType(machineType string) bool {
	// This is a placeholder. In a real scenario, you would check against a list of valid Azure machine types.
	validTypes, err := GetMachineTypes()
	if err != nil {
		return false
	}
	if machineType == "" {
		return false
	}
	for _, validType := range validTypes {
		if machineType == validType {
			return true
		}
	}
	return false
}
