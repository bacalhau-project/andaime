package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bacalhau-project/andaime/utils"
	"github.com/spf13/viper"
)

var requiredFields = []string{
	"general.project_id",
	"general.ssh_public_key_path",
}

func validateConfig(v *viper.Viper) error {
	missingFields := []string{}
	for _, field := range requiredFields {
		if !v.IsSet(field) {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missingFields, ", "))
	}

	// If v.unique_id is not set, then create it and save it to the config. It should be a short (8-character) lowercase and numbers ascii only string.
	if !v.IsSet("general.unique_id") {
		v.Set("general.unique_id", utils.GenerateUniqueID())
	}

	// Validate SSH public key file exists
	sshKeyPath := v.GetString("general.ssh_public_key_path")
	if _, err := os.ReadFile(sshKeyPath); err != nil {
		return fmt.Errorf("unable to read SSH public key file: %v", err)
	}

	return nil
}

func validateConfigValues(v *viper.Viper) error {
	var errors []string

	// Validate Azure configuration
	if v.IsSet("azure") {
		azureErrors := validateAzureConfig(v.Sub("azure"))
		errors = append(errors, azureErrors...)
	}

	// Add validation for other cloud providers here (AWS, GCP, etc.)

	if len(errors) > 0 {
		return fmt.Errorf("config validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

func validateAzureConfig(v *viper.Viper) []string {
	var errors []string

	// Validate locations
	locations := v.Get("locations").([]interface{})
	if len(locations) == 0 {
		errors = append(errors, "at least one location must be specified for Azure")
	}

	for i, loc := range locations {
		location := loc.(map[string]interface{})
		if _, ok := location["name"]; !ok {
			errors = append(errors, fmt.Sprintf("location %d is missing 'name'", i))
		}
		if zones, ok := location["zones"]; ok {
			zoneErrors := validateZones(zones.([]interface{}))
			errors = append(errors, zoneErrors...)
		} else {
			errors = append(errors, fmt.Sprintf("location %d is missing 'zones'", i))
		}
	}

	// Validate allowed ports
	allowedPorts := v.GetIntSlice("allowed_ports")
	for _, port := range allowedPorts {
		if port < 1 || port > 65535 {
			errors = append(errors, fmt.Sprintf("invalid port number: %d", port))
		}
	}

	// Validate disk size
	diskSizeGB := v.GetInt("disk_size_gb")
	if diskSizeGB <= 0 {
		errors = append(errors, "disk size must be positive")
	}

	return errors
}

func validateZones(zones []interface{}) []string {
	var errors []string

	for i, z := range zones {
		zone := z.(map[string]interface{})
		if _, ok := zone["name"]; !ok {
			errors = append(errors, fmt.Sprintf("zone %d is missing 'name'", i))
		}
		if machines, ok := zone["machines"]; ok {
			machineErrors := validateMachines(machines.([]interface{}))
			errors = append(errors, machineErrors...)
		} else {
			errors = append(errors, fmt.Sprintf("zone %d is missing 'machines'", i))
		}
	}

	return errors
}

func validateMachines(machines []interface{}) []string {
	var errors []string

	for i, m := range machines {
		machine := m.(map[string]interface{})
		if _, ok := machine["type"]; !ok {
			errors = append(errors, fmt.Sprintf("machine %d is missing 'type'", i))
		}
		if count, ok := machine["count"]; ok {
			if count.(int) < 1 {
				errors = append(errors, fmt.Sprintf("machine %d has invalid count: %d", i, count))
			}
		} else {
			errors = append(errors, fmt.Sprintf("machine %d is missing 'count'", i))
		}
	}

	return errors
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
