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

type DiskImage struct {
	Name         string   `json:"name"         yaml:"name"`
	Family       string   `json:"family"       yaml:"family"`
	Description  string   `json:"description"  yaml:"description"`
	DiskSizeGb   string   `json:"diskSizeGb"   yaml:"diskSizeGb"`
	Licenses     []string `json:"licenses"     yaml:"licenses"`
	Architecture string   `json:"architecture" yaml:"architecture"`
}

type CloudData struct {
	Locations    map[string][]string `yaml:"locations"`
	DiskImages   []DiskImage         `yaml:"diskImages"`
	RemovedZones []string            `yaml:"removedZones"`
}

type lineSpinner struct {
	*spinner.Spinner
	line int
}

func newLineSpinner(line int, suffix string) *lineSpinner {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = suffix
	return &lineSpinner{Spinner: s, line: line}
}

func (ls *lineSpinner) Start() {
	go func() {
		for {
			ls.update()
			time.Sleep(ls.Delay)
			if !ls.Active() {
				break
			}
		}
	}()
}

func (ls *lineSpinner) Stop() {
	ls.Spinner.Stop()
	ls.update()
}

func (ls *lineSpinner) UpdateSuffix(suffix string) {
	ls.Spinner.Suffix = suffix
	ls.update()
}

func (ls *lineSpinner) update() {
	fmt.Printf(
		"\033[%dA\r%s%s\033[K\033[%dB",
		ls.line,
		ls.Spinner.Prefix,
		ls.Spinner.Suffix,
		ls.line,
	)
}

func main() {
	var wg sync.WaitGroup
	errorChan := make(chan error, 2)
	removedZones := make(chan string, 100)

	fmt.Println("Starting cloud data generation:")
	fmt.Println("Azure:")
	fmt.Println("GCP:")
	fmt.Println("GCP Images:")

	// Create spinners
	azureSpinner := newLineSpinner(1, " Initializing...")
	gcpSpinner := newLineSpinner(2, " Initializing...")
	gcpImagesSpinner := newLineSpinner(3, " Initializing...")

	azureSpinner.Start()
	gcpSpinner.Start()
	gcpImagesSpinner.Start()

	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := generateAzureData(removedZones, azureSpinner); err != nil {
			errorChan <- fmt.Errorf("Error generating Azure data: %v", err)
		}
		azureSpinner.Stop()
	}()

	go func() {
		defer wg.Done()
		if err := generateGCPData(removedZones, gcpSpinner, gcpImagesSpinner); err != nil {
			errorChan <- fmt.Errorf("Error generating GCP data: %v", err)
		}
		gcpSpinner.Stop()
		gcpImagesSpinner.Stop()
	}()

	wg.Wait()
	close(errorChan)
	close(removedZones)

	fmt.Println() // Move cursor below the spinners

	// Print errors
	for err := range errorChan {
		log.Println(err)
	}

	// Print removed zones
	fmt.Println("\nRemoved zones:")
	for zone := range removedZones {
		fmt.Println(zone)
	}
}

func generateAzureData(removedZones chan<- string, s *lineSpinner) error {
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

	azureData := CloudData{
		Locations:    make(map[string][]string),
		RemovedZones: make([]string, 0),
	}
	var wg sync.WaitGroup
	locationChan := make(chan string, len(azureLocations))
	resultChan := make(chan struct {
		location string
		vmSizes  []string
		err      error
	}, len(azureLocations))

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
				removedZone := fmt.Sprintf("Azure: %s", result.location)
				removedZones <- removedZone
				azureData.RemovedZones = append(azureData.RemovedZones, removedZone)
				s.UpdateSuffix(
					fmt.Sprintf(
						" Fetching Azure zone and VM sizes... (%d/%d) Error: %s",
						processedCount,
						len(azureLocations),
						result.location,
					),
				)
			} else {
				s.UpdateSuffix(
					fmt.Sprintf(
						" Fetching Azure zone and VM sizes... (%d/%d) Error: %s",
						processedCount,
						len(azureLocations),
						result.err,
					),
				)
			}
		} else {
			azureData.Locations[result.location] = result.vmSizes
			s.UpdateSuffix(
				fmt.Sprintf(
					" Fetching Azure zone and VM sizes... (%d/%d)",
					processedCount,
					len(azureLocations),
				),
			)
		}
		processedCount++
	}

	s.UpdateSuffix(
		fmt.Sprintf(
			" Completed fetching Azure zone and VM sizes. (%d/%d). %d removed zones.",
			processedCount,
			len(azureLocations),
			len(azureData.RemovedZones),
		),
	)

	// Sort locations and machine types
	sortedLocations := make([]string, 0, len(azureData.Locations))
	for location := range azureData.Locations {
		sortedLocations = append(sortedLocations, location)
	}
	sort.Strings(sortedLocations)

	sortedAzureData := CloudData{
		Locations:    make(map[string][]string),
		RemovedZones: make([]string, 0),
	}
	for _, location := range sortedLocations {
		machineTypes := azureData.Locations[location]
		sort.Strings(machineTypes)
		sortedAzureData.Locations[location] = machineTypes
	}

	sortedAzureData.RemovedZones = azureData.RemovedZones

	yamlData, err := yaml.Marshal(sortedAzureData)
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
			return nil, fmt.Errorf("unsupported location")
		}
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
	for _, vmSize := range vmSizes {
		topVMSizes = append(topVMSizes, vmSize.Name)
	}
	return topVMSizes, nil
}

func generateGCPData(removedZones chan<- string, s *lineSpinner, imagesSpinner *lineSpinner) error {
	var wg sync.WaitGroup
	var gcpDataMutex sync.Mutex
	gcpData := CloudData{
		Locations:    make(map[string][]string),
		DiskImages:   make([]DiskImage, 0),
		RemovedZones: make([]string, 0),
	}

	wg.Add(2)

	// Fetch zones and machine types
	go func() {
		defer wg.Done()
		_, machineTypes, err := fetchGCPZonesAndMachineTypes(s)
		if err != nil {
			s.UpdateSuffix(fmt.Sprintf(" Error fetching zones and machine types: %v", err))
			return
		}
		gcpDataMutex.Lock()
		gcpData.Locations = machineTypes
		gcpDataMutex.Unlock()
	}()

	// Fetch disk images
	go func() {
		defer wg.Done()
		diskImages, err := generateGCPDiskImages(imagesSpinner)
		if err != nil {
			imagesSpinner.UpdateSuffix(
				fmt.Sprintf(" Error fetching disk images: %v", err),
			)
			return
		}
		gcpDataMutex.Lock()
		gcpData.DiskImages = diskImages
		gcpDataMutex.Unlock()
	}()

	wg.Wait()

	// Sort locations and machine types
	sortedLocations := make([]string, 0, len(gcpData.Locations))
	for location := range gcpData.Locations {
		sortedLocations = append(sortedLocations, location)
	}
	sort.Strings(sortedLocations)

	sortedGCPData := CloudData{
		Locations:    make(map[string][]string),
		DiskImages:   gcpData.DiskImages,
		RemovedZones: gcpData.RemovedZones,
	}
	for _, location := range sortedLocations {
		machineTypes := gcpData.Locations[location]
		sort.Strings(machineTypes)
		sortedGCPData.Locations[location] = machineTypes
	}

	// Create a more compact YAML representation
	yamlData, err := yaml.Marshal(sortedGCPData)
	if err != nil {
		return fmt.Errorf("failed to marshal GCP data to YAML: %v", err)
	}

	// Write the YAML data to file
	if err := os.WriteFile("internal/clouds/gcp/gcp_data.yaml", yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write GCP YAML file: %v", err)
	}

	s.UpdateSuffix(
		fmt.Sprintf(
			" GCP data generated: %d zones, %d disk images",
			len(sortedGCPData.Locations),
			len(sortedGCPData.DiskImages),
		),
	)
	return nil
}

func fetchGCPZonesAndMachineTypes(s *lineSpinner) ([]string, map[string][]string, error) {
	gcpZonesCmd := exec.Command("sh", "-c", "gcloud compute zones list --format=json")
	gcpZonesOutput, err := gcpZonesCmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get GCP zones: %v", err)
	}

	var gcpZones []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(gcpZonesOutput, &gcpZones); err != nil {
		return nil, nil, fmt.Errorf("failed to parse GCP zones data: %v", err)
	}

	zones := make([]string, len(gcpZones))
	for i, zone := range gcpZones {
		zones[i] = zone.Name
	}

	machineTypes := make(map[string][]string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent requests

	lenZones := len(zones)
	currentZoneTotal := 0
	for i, zone := range zones {
		wg.Add(1)
		go func(i int, zone string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			types, err := getGCPMachineTypes(zone)
			if err != nil {
				s.UpdateSuffix(
					fmt.Sprintf(
						" Error fetching machine types for zone %s: %v",
						zone,
						err,
					),
				)
			} else {
				mu.Lock()
				currentZoneTotal++
				machineTypes[zone] = types
				mu.Unlock()
			}
			s.UpdateSuffix(
				fmt.Sprintf(
					" Fetching machine types... (%d/%d) Zone: %s",
					currentZoneTotal,
					lenZones,
					zone,
				),
			)
		}(i, zone)
	}

	wg.Wait()

	return zones, machineTypes, nil
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
	for _, machineType := range gcpMachineTypes {
		machineTypesList = append(machineTypesList, machineType.Name)
	}
	return machineTypesList, nil
}

func generateGCPDiskImages(s *lineSpinner) ([]DiskImage, error) {
	s.UpdateSuffix(" Fetching disk images...")
	gcpDiskImagesCmd := exec.Command(
		"sh",
		"-c",
		"gcloud compute images list --format=json",
	)
	gcpDiskImagesOutput, err := gcpDiskImagesCmd.Output()
	if err != nil {
		s.UpdateSuffix(fmt.Sprintf(" Error fetching disk images: %v", err))
		return nil, fmt.Errorf("failed to get GCP disk images: %v", err)
	}

	s.UpdateSuffix(" Parsing disk images...")
	var diskImages []DiskImage
	if err := json.Unmarshal(gcpDiskImagesOutput, &diskImages); err != nil {
		s.UpdateSuffix(fmt.Sprintf(" Error parsing disk images: %v", err))
		return nil, fmt.Errorf("failed to parse GCP disk images data: %v", err)
	}
	s.UpdateSuffix(fmt.Sprintf(" Fetched %d disk images", len(diskImages)))

	return diskImages, nil
}
