package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/briandowns/spinner"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

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
	DiskImages   []DiskImage         `yaml:"diskImages,omitempty"`
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
			time.Sleep(ls.Spinner.Delay)
			if !ls.Spinner.Active() {
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

// GenerateCloudDataCmd is the cobra command for generating multi-cloud data.
var GenerateCloudDataCmd = &cobra.Command{
	Use:   "generate-cloud-data",
	Short: "Generate multi-cloud data for AWS, Azure, and GCP",
	Run: func(cmd *cobra.Command, args []string) {
		GenerateCloudData()
	},
}

func GetGenerateCloudDataCmd() *cobra.Command {
	return GenerateCloudDataCmd
}

// GenerateCloudData generates the multi-cloud data for AWS, Azure, and GCP.
func GenerateCloudData() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		return
	}

	// Verify that required AWS environment variables are set
	requiredEnvVars := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION"}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			fmt.Printf(
				"Error: %s is not set. Please check your .env file or environment variables.\n",
				envVar,
			)
			return
		}
	}

	fmt.Println("Starting cloud data generation:")
	fmt.Println("Azure:")
	fmt.Println("GCP:")
	fmt.Println("AWS:")

	// Create spinners
	azureSpinner := newLineSpinner(1, " Initializing...")
	gcpSpinner := newLineSpinner(2, " Initializing...") //nolint:mnd
	awsSpinner := newLineSpinner(3, " Initializing...") //nolint:mnd

	azureSpinner.Start()
	gcpSpinner.Start()
	awsSpinner.Start()

	var wg sync.WaitGroup
	errorChan := make(chan error, 3)       //nolint:gomnd
	removedZones := make(chan string, 100) //nolint:gomnd

	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := generateAzureData(removedZones, azureSpinner); err != nil {
			errorChan <- fmt.Errorf("error generating Azure data: %v", err)
		}
		azureSpinner.Stop()
	}()

	go func() {
		defer wg.Done()
		if err := generateGCPData(removedZones, gcpSpinner); err != nil {
			errorChan <- fmt.Errorf("error generating GCP data: %v", err)
		}
		gcpSpinner.Stop()
	}()

	go func() {
		defer wg.Done()
		if err := generateAWSData(removedZones, awsSpinner); err != nil {
			errorChan <- fmt.Errorf("error generating AWS data: %v", err)
		}
		awsSpinner.Stop()
	}()

	wg.Wait()
	close(errorChan)
	close(removedZones)

	fmt.Println() // Move cursor below the spinners

	// Print errors
	for err := range errorChan {
		fmt.Println(err)
	}

	// Print removed zones
	fmt.Println("\nRemoved zones:")
	for zone := range removedZones {
		fmt.Println(zone)
	}
}

func generateAWSData(removedZones chan<- string, s *lineSpinner) error {
	awsLocationsCmd := exec.Command(
		"aws",
		"ec2",
		"describe-regions",
		"--query",
		"Regions[].RegionName",
		"--output",
		"json",
	)
	awsLocationsOutput, err := awsLocationsCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get AWS locations: %v", err)
	}

	var awsLocations []string
	if err := json.Unmarshal(awsLocationsOutput, &awsLocations); err != nil {
		return fmt.Errorf("failed to parse AWS locations data: %v", err)
	}

	sort.Strings(awsLocations)
	awsData := CloudData{
		Locations: make(map[string][]string),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(awsLocations))
	locationChan := make(chan string, len(awsLocations))

	// Start worker goroutines
	workerCount := 10
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for location := range locationChan {
				if err != nil {
					errorChan <- fmt.Errorf("failed to get Ubuntu AMI for region %s: %v", location, err)
					continue
				}

				vmSizes, err := getAWSVMSizes(location)
				if err != nil {
					errorChan <- fmt.Errorf("failed to get VM sizes for region %s: %v", location, err)
					continue
				}

				mu.Lock()
				awsData.Locations[location] = vmSizes
				mu.Unlock()
			}
		}()
	}

	// Send locations to be processed
	for _, location := range awsLocations {
		locationChan <- location
	}
	close(locationChan)

	// Collect results
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	processedCount := 0
	errorLog := make([]string, 0)
	for err := range errorChan {
		if err != nil {
			errorLog = append(errorLog, err.Error())
			removedZones <- err.Error()
		} else {
			processedCount++
		}
		s.UpdateSuffix(
			fmt.Sprintf(" Processing AWS regions... (%d/%d)", processedCount, len(awsLocations)),
		)
	}

	// Log errors to a file
	if len(errorLog) > 0 {
		errorLogContent := strings.Join(errorLog, "\n")
		if err := os.WriteFile("aws_errors.log", []byte(errorLogContent), 0600); err != nil {
			fmt.Printf("Failed to write AWS error log: %v\n", err)
		}
	}

	yamlData, err := yaml.Marshal(awsData)
	if err != nil {
		return fmt.Errorf("failed to marshal AWS data to YAML: %v", err)
	}

	if err := os.WriteFile("./internal/clouds/aws/aws_data.yaml", yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write AWS YAML file: %v", err)
	}

	s.UpdateSuffix(
		fmt.Sprintf(" Completed fetching AWS regions (%d/%d)", processedCount, len(awsLocations)),
	)

	return nil
}

func getAWSVMSizes(region string) ([]string, error) {
	command := fmt.Sprintf(
		"aws ec2 describe-instance-types --region %s --query 'InstanceTypes[].{name:InstanceType, cpus:VCpuInfo.DefaultVCpus}' --output json",
		region,
	)
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get VM sizes for region %s: %v\nCommand output: %s",
			region,
			err,
			string(output),
		)
	}

	var vmSizes []struct {
		Name string `json:"name"`
		Cpus int    `json:"cpus"`
	}
	if err := json.Unmarshal(output, &vmSizes); err != nil {
		return nil, fmt.Errorf("failed to parse VM sizes for AWS region %s: %v", region, err)
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

func getUbuntuAMI(region string) (string, error) {
	l := logger.Get()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	command := fmt.Sprintf(
		`aws ec2 describe-images --region %s --owners 099720109477 --filters "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*" "Name=state,Values=available" --query "sort_by(Images, &CreationDate)[-1].ImageId" --output text`,
		region,
	)

	l.Debugf("Running command: %s", command)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out for region %s", region)
	}

	if err != nil {
		if strings.Contains(string(output), "AuthFailure") {
			return "", fmt.Errorf("authentication failed for region %s, skipping", region)
		}
		return "", fmt.Errorf(
			"failed to get Ubuntu AMI for region %s: %v\nCommand output: %s",
			region,
			err,
			string(output),
		)
	}

	amiID := strings.TrimSpace(string(output))
	if amiID == "" {
		return "", fmt.Errorf("no valid Ubuntu AMI found for region %s", region)
	}

	return amiID, nil
}

func generateAzureData(removedZones chan<- string, s *lineSpinner) error {
	azureLocationsCmd := exec.Command(
		"az",
		"account",
		"list-locations",
		"--query",
		"[].name",
		"-o",
		"json",
	)
	azureLocationsOutput, err := azureLocationsCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get Azure locations: %v", err)
	}

	var azureLocations []string
	if err := json.Unmarshal(azureLocationsOutput, &azureLocations); err != nil {
		return fmt.Errorf("failed to parse Azure locations data: %v", err)
	}

	sort.Strings(azureLocations)
	azureData := CloudData{
		Locations: make(map[string][]string),
	}

	var wg sync.WaitGroup
	locationChan := make(chan string, len(azureLocations))
	resultChan := make(chan struct {
		location string
		vmSizes  []string
		err      error
	}, len(azureLocations))

	// Start worker goroutines
	workerCount := 10
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
	for _, location := range azureLocations {
		locationChan <- location
	}
	close(locationChan)

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	processedCount := 0
	for result := range resultChan {
		if result.err != nil {
			removedZones <- fmt.Sprintf("Azure: %s", result.location)
			azureData.RemovedZones = append(azureData.RemovedZones, result.location)
		} else {
			azureData.Locations[result.location] = result.vmSizes
		}
		processedCount++
		s.UpdateSuffix(
			fmt.Sprintf(
				" Fetching Azure zone and VM sizes... (%d/%d)",
				processedCount,
				len(azureLocations),
			),
		)
	}

	yamlData, err := yaml.Marshal(azureData)
	if err != nil {
		return fmt.Errorf("failed to marshal Azure data to YAML: %v", err)
	}

	if err := os.WriteFile("./internal/clouds/azure/azure_data.yaml", yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write Azure YAML file: %v", err)
	}

	s.UpdateSuffix(
		fmt.Sprintf(
			" Completed fetching Azure zone and VM sizes. (%d/%d). %d removed zones.",
			processedCount,
			len(azureLocations),
			len(azureData.RemovedZones),
		),
	)

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

func generateGCPData(removedZones chan<- string, s *lineSpinner) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	gcpZonesCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"zones",
		"list",
		"--format",
		"json",
	)
	gcpZonesOutput, err := gcpZonesCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get GCP zones: %v", err)
	}

	var gcpZones []struct {
		Name   string `json:"name"`
		Region string `json:"region"`
	}
	if err := json.Unmarshal(gcpZonesOutput, &gcpZones); err != nil {
		return fmt.Errorf("failed to parse GCP zones data: %v", err)
	}

	zones := make([]string, len(gcpZones))
	for i, zone := range gcpZones {
		zones[i] = zone.Name
	}

	sort.Strings(zones)
	gcpData := CloudData{
		Locations:    make(map[string][]string),
		DiskImages:   make([]DiskImage, 0),
		RemovedZones: make([]string, 0),
	}

	// Fetch all machine types in a single query
	s.UpdateSuffix(" Fetching GCP machine types for all zones...")
	allMachineTypes, err := getAllGCPMachineTypes()
	if err != nil {
		return fmt.Errorf("failed to get all GCP machine types: %v", err)
	}

	// Process the machine types for each zone
	for _, zone := range zones {
		vmSizes, exists := allMachineTypes[zone]
		if !exists || len(vmSizes) == 0 {
			errorMsg := fmt.Sprintf("GCP: %s - Warning: No VM sizes found", zone)
			removedZones <- errorMsg
			gcpData.RemovedZones = append(gcpData.RemovedZones, zone)
			fmt.Println(errorMsg) // Print warning message to console
		} else {
			gcpData.Locations[zone] = vmSizes
		}
		s.UpdateSuffix(
			fmt.Sprintf(
				" Processing GCP zone and VM sizes... (%d/%d)",
				len(gcpData.Locations),
				len(zones),
			),
		)
	}

	// Fetch GCP disk images
	s.UpdateSuffix(" Fetching GCP disk images...")
	diskImages, err := getGCPDiskImagesWithRetry(3) // Retry up to 3 times
	if err != nil {
		return fmt.Errorf("failed to get GCP disk images: %v", err)
	}
	gcpData.DiskImages = diskImages

	yamlData, err := yaml.Marshal(gcpData)
	if err != nil {
		return fmt.Errorf("failed to marshal GCP data to YAML: %v", err)
	}

	if err := os.WriteFile("./internal/clouds/gcp/gcp_data.yaml", yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write GCP YAML file: %v", err)
	}

	s.UpdateSuffix(
		fmt.Sprintf(
			" Completed fetching GCP data. (%d/%d zones). %d removed zones.",
			len(gcpData.Locations),
			len(zones),
			len(gcpData.RemovedZones),
		),
	)

	return nil
}

func getAllGCPMachineTypes() (map[string][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	command := "gcloud compute machine-types list --format='json(name,zone)'"
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get all GCP machine types: %v\nCommand output: %s",
			err,
			string(output),
		)
	}

	var machineTypes []struct {
		Name string `json:"name"`
		Zone string `json:"zone"`
	}
	if err := json.Unmarshal(output, &machineTypes); err != nil {
		return nil, fmt.Errorf("failed to parse GCP machine types: %v", err)
	}

	result := make(map[string][]string)
	for _, mt := range machineTypes {
		zone := strings.TrimPrefix(
			mt.Zone,
			"https://www.googleapis.com/compute/v1/projects/[PROJECT]/zones/",
		)
		result[zone] = append(result[zone], mt.Name)
	}

	// Sort the machine types for each zone
	for zone := range result {
		sort.Strings(result[zone])
	}

	return result, nil
}

func getGCPDiskImagesWithRetry(maxRetries int) ([]DiskImage, error) {
	var (
		diskImages []DiskImage
		err        error
	)

	for i := 0; i < maxRetries; i++ {
		diskImages, err = getGCPDiskImages()
		if err == nil {
			return diskImages, nil
		}
		time.Sleep(time.Second * time.Duration(i+1)) // Exponential backoff
	}

	return nil, fmt.Errorf("failed to get disk images after %d retries: %v", maxRetries, err)
}

func getGCPDiskImages() ([]DiskImage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	command := `gcloud compute images list --format='json(name,family,description,diskSizeGb,licenses,architecture)'`
	gcpDiskImagesCmd := exec.CommandContext(ctx, "sh", "-c", command)
	gcpDiskImagesOutput, err := gcpDiskImagesCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get GCP disk images: %v", err)
	}

	var diskImages []DiskImage
	if err := json.Unmarshal(gcpDiskImagesOutput, &diskImages); err != nil {
		return nil, fmt.Errorf("failed to parse GCP disk images: %v", err)
	}

	return diskImages, nil
}
