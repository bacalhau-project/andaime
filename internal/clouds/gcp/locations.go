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
func IsValidGCPDiskImageFamily(location, diskImageFamilyToCheck string) (string, error) {
	l := logger.Get()
	gcpDataRaw, err := GetGCPData()
	if err != nil {
		return "", err
	}

	var gcpData GCPData
	err = yaml.Unmarshal(gcpDataRaw, &gcpData)
	if err != nil {
		l.Warnf("Failed to unmarshal GCP data: %v", err)
		return "", err
	}

	for _, diskImage := range gcpData.DiskImages {
		if diskImage.Family == diskImageFamilyToCheck {
			// Extract the project ID (e.g., "ubuntu-os-cloud") from the license URL
			if len(diskImage.Licenses) > 0 {
				parts := strings.Split(diskImage.Licenses[0], "/")
				//nolint:mnd
				if len(parts) >= 6 {
					projectID := parts[6] // e.g., "ubuntu-os-cloud"
					imageURL := GetGCPDiskImageURL(projectID, diskImage.Family)
					return imageURL, nil
				}
			}
			return "", fmt.Errorf("invalid disk image family for GCP: %s", diskImageFamilyToCheck)
		}
	}
	return "", fmt.Errorf("invalid disk image family for GCP: %s", diskImageFamilyToCheck)
}

func GetGCPDiskImageURL(projectID, family string) string {
	return fmt.Sprintf(
		"https://www.googleapis.com/compute/v1/projects/%s/global/images/family/%s",
		projectID,
		family,
	)
}

func GetGCPRegionFromZone(zone string) (string, error) {
	parts := strings.Split(zone, "-")
	//nolint:gomnd
	if len(parts) >= 3 {
		return fmt.Sprintf("%s-%s", parts[0], parts[1]), nil
	}
	return "", fmt.Errorf("invalid zone for GCP: %s", zone)
}
