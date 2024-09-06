package internal_gcp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"gopkg.in/yaml.v2"
)

type GCPData struct {
	Locations  map[string][]string `yaml:"locations"`
	DiskImages []DiskImage         `yaml:"diskImages"`
}

type DiskImage struct {
	Name         string   `yaml:"name"`
	Family       string   `yaml:"family"`
	Description  string   `yaml:"description"`
	DiskSizeGb   string   `yaml:"diskSizeGb"`
	Licenses     []string `yaml:"licenses"`
	Architecture string   `yaml:"architecture"`
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
	validMachineTypes, ok := gcpData.Locations[strings.ToLower(location)]
	if !ok {
		return false
	}
	return slices.Contains(validMachineTypes, machineType)
}

// Returns the name of the disk image, if the disk image family is valid, otherwise returns an empty string
func IsValidGCPDiskImageFamily(location, diskImageFamilyToCheck string) error {
	l := logger.Get()
	gcpDataRaw, err := GetGCPData()
	if err != nil {
		return err
	}

	var gcpData GCPData
	err = yaml.Unmarshal(gcpDataRaw, &gcpData)
	if err != nil {
		l.Warnf("Failed to unmarshal GCP data: %v", err)
		return err
	}

	for _, diskImage := range gcpData.DiskImages {
		if diskImage.Family == diskImageFamilyToCheck {
			return nil
		}
	}
	return fmt.Errorf("invalid disk image family for GCP: %s", diskImageFamilyToCheck)
}
