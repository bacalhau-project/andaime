//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"strings"

	"github.com/briandowns/spinner"
	"gopkg.in/yaml.v2"
)

const topMachineTypes = 10

type CloudData struct {
	Locations map[string][]string `yaml:"locations"`
}

func main() {
	if err := generateGCPData(); err != nil {
		log.Printf("Error generating GCP data: %v", err)
	}
	if err := generateAzureData(); err != nil {
		log.Printf("Error generating Azure data: %v", err)
	}
}

func generateGCPData() error {
	gcpZonesCmd := exec.Command("sh", "-c", "gcloud compute zones list --format=json")
	gcpZonesOutput, err := gcpZonesCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get GCP zones: %v", err)
	}

	var gcpZones []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(gcpZonesOutput, &gcpZones); err != nil {
		return fmt.Errorf("failed to parse GCP zones data: %v", err)
	}

	gcpData := CloudData{Locations: make(map[string][]string)}

	var wg sync.WaitGroup
	zoneChan := make(chan string, len(gcpZones))
	resultChan := make(chan struct {
		zone         string
		machineTypes []string
		err          error
	}, len(gcpZones))

	// Create and start spinner
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = " Fetching GCP machine types..."
	s.Start()

	// Start worker goroutines
	workerCount := 10
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for zone := range zoneChan {
				machineTypes, err := getGCPMachineTypes(zone)
				resultChan <- struct {
					zone         string
					machineTypes []string
					err          error
				}{zone, machineTypes, err}
			}
		}()
	}

	// Send zones to be processed
	go func() {
		for _, zone := range gcpZones {
			zoneChan <- zone.Name
		}
		close(zoneChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	processedCount := 0
	for result := range resultChan {
		if result.err != nil {
			log.Printf(
				"Warning: Failed to get machine types for GCP zone %s: %v",
				result.zone,
				result.err,
			)
		} else {
			gcpData.Locations[result.zone] = result.machineTypes
		}
		processedCount++
		s.Suffix = fmt.Sprintf(
			" Fetching GCP machine types... (%d/%d)",
			processedCount,
			len(gcpZones),
		)
	}

	// Stop spinner
	s.Stop()

	yamlData, err := yaml.Marshal(gcpData)
	if err != nil {
		return fmt.Errorf("failed to marshal GCP data to YAML: %v", err)
	}

	if err := os.WriteFile("internal/clouds/gcp/gcp_data.yaml", yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write GCP YAML file: %v", err)
	}

	return nil
}

func getGCPMachineTypes(zone string) ([]string, error) {
	gcpMachineTypesCmd := exec.Command(
		"sh",
		"-c",
		fmt.Sprintf("gcloud compute machine-types list --zones %s --format=json", zone),
	)
	gcpMachineTypesOutput, err := gcpMachineTypesCmd.Output()
	if err != nil {
		return nil, err
	}

	var gcpMachineTypes []struct {
		Name string `json:"name"`
		Cpus int    `json:"guestCpus"`
	}
	if err := json.Unmarshal(gcpMachineTypesOutput, &gcpMachineTypes); err != nil {
		return nil, err
	}

	sort.Slice(gcpMachineTypes, func(i, j int) bool {
		return gcpMachineTypes[i].Cpus > gcpMachineTypes[j].Cpus
	})

	var machineTypesList []string
	for i, machineType := range gcpMachineTypes {
		if i >= topMachineTypes {
			break
		}
		machineTypesList = append(machineTypesList, machineType.Name)
	}
	return machineTypesList, nil
}

func generateAzureData() error {
	azureLocationsCmd := exec.Command(
		"sh",
		"-c",
		"az account list-locations --query '[].name' -o json",
	)
	azureLocationsOutput, err := azureLocationsCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get Azure locations: %v", err)
	}

	var azureLocations []string
	if err := json.Unmarshal(azureLocationsOutput, &azureLocations); err != nil {
		return fmt.Errorf("failed to parse Azure locations data: %v", err)
	}

	azureData := CloudData{Locations: make(map[string][]string)}
	var wg sync.WaitGroup
	locationChan := make(chan string, len(azureLocations))
	resultChan := make(chan struct {
		location string
		vmSizes  []string
		err      error
	}, len(azureLocations))

	// Create and start spinner
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = " Fetching Azure VM sizes..."
	s.Start()

	// Start worker goroutines
	workerCount := 10 // Adjust this number based on your needs
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for location := range locationChan {
				vmSizes, err := getAzureVMSizes(location)
				resultChan <- struct {
					location string
					vmSizes  []string
					err      error
				}{location, vmSizes, err}
			}
		}()
	}

	// Send locations to be processed
	go func() {
		for _, location := range azureLocations {
			locationChan <- location
		}
		close(locationChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	processedCount := 0
	for result := range resultChan {
		if result.err != nil {
			if result.err.Error() == "unsupported location" {
				// Location was already logged as unsupported, so we skip it silently here
			} else {
				log.Printf(
					"Warning: Failed to get VM sizes for Azure location %s: %v",
					result.location,
					result.err,
				)
			}
		} else {
			azureData.Locations[result.location] = result.vmSizes
		}
		processedCount++
		s.Suffix = fmt.Sprintf(
			" Fetching Azure VM sizes... (%d/%d)",
			processedCount,
			len(azureLocations),
		)
	}

	// Stop spinner
	s.Stop()

	yamlData, err := yaml.Marshal(azureData)
	if err != nil {
		return fmt.Errorf("failed to marshal Azure data to YAML: %v", err)
	}

	if err := os.WriteFile("./internal/clouds/azure/azure_data.yaml", yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write Azure YAML file: %v", err)
	}

	return nil
}

func getAzureVMSizes(location string) ([]string, error) {
	command := fmt.Sprintf(
		"az vm list-sizes --location %s --query '[].{name:name, cpus:numberOfCores}' -o json",
		location,
	)
	azureVMSizesCmd := exec.Command("sh", "-c", command)
	azureVMSizesOutput, err := azureVMSizesCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(azureVMSizesOutput), "NoRegisteredProviderFound") {
			log.Printf("Warning: Removing unsupported Azure location: %s", location)
			return nil, fmt.Errorf("unsupported location")
		}
		log.Printf("Command failed: %s", command)
		log.Printf("Error output: %s", string(azureVMSizesOutput))
		return nil, fmt.Errorf("command execution failed: %v", err)
	}

	var vmSizes []struct {
		Name string `json:"name"`
		Cpus int    `json:"cpus"`
	}
	if err := json.Unmarshal(azureVMSizesOutput, &vmSizes); err != nil {
		return nil, fmt.Errorf("failed to parse VM sizes for Azure location %s: %v", location, err)
	}

	sort.Slice(vmSizes, func(i, j int) bool {
		return vmSizes[i].Cpus > vmSizes[j].Cpus
	})

	var topVMSizes []string
	for i, vmSize := range vmSizes {
		if i >= topMachineTypes {
			break
		}
		topVMSizes = append(topVMSizes, vmSize.Name)
	}
	return topVMSizes, nil
}
