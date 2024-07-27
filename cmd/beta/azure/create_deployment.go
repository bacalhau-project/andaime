package azure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type contextKey string

const uniqueDeploymentIDKey contextKey = "UniqueDeploymentID"

var createAzureDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in Azure",
	Long:  `Create a deployment in Azure using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func GetAzureCreateDeploymentCmd() *cobra.Command {
	return createAzureDeploymentCmd
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	logger.InitProduction(false, true)
	l := logger.Get()

	l.Debug("Starting executeCreateDeployment")

	// Create a unique ID for the deployment
	UniqueID := fmt.Sprintf(
		"%s-%s",
		viper.GetString("general.project_id"),
		time.Now().Format("060102150405"),
	)

	// Set the UniqueID on the context
	ctx := context.WithValue(cmd.Context(), uniqueDeploymentIDKey, UniqueID)

	l.Debug("Initializing Azure provider")
	azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
	if err != nil {
		errString := fmt.Sprintf("Failed to initialize Azure provider: %s", err.Error())
		l.Error(errString)
		return fmt.Errorf(errString)
	}
	l.Debug("Azure provider initialized successfully")

	l.Debug("Setting up signal channel")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	l.Debug("Creating display")
	disp := display.NewDisplay(1)
	l.Debug("Starting display")
	go func() {
		l.Debug("Display Start() called")
		disp.Start(sigChan)
		l.Debug("Display Start() returned")
	}()

	defer func() {
		l.Debug("Stopping display")
		disp.Stop()
		l.Debug("Display stopped")
	}()

	l.Debug("Updating initial status")

	// Create a new deployment object
	deployment, err := InitializeDeployment(ctx, UniqueID, disp)
	if err != nil {
		return err
	}
	l.Debug("Starting resource deployment")
	err = azureProvider.DeployResources(ctx, deployment, disp)
	if err != nil {
		errString := fmt.Sprintf("Failed to deploy resources: %s", err.Error())
		l.Error(errString)
		return fmt.Errorf(errString)
	}

	l.Debug("Resource deployment completed")

	l.Info("Azure deployment created successfully")
	cmd.Println("Azure deployment created successfully")
	cmd.Println("Press 'q' and Enter to quit")

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			l.Debug("Waiting for input")
			char, _, err := reader.ReadRune()
			if err != nil {
				l.Error(fmt.Sprintf("Error reading input: %s", err.Error()))
				continue
			}
			if char == 'q' || char == 'Q' {
				l.Debug("Quit signal received")
				sigChan <- os.Interrupt
				return
			}
		}
	}()

	l.Debug("Waiting for signal")
	<-sigChan
	l.Debug("Signal received, exiting")
	return nil
}

// initializeDeployment prepares the deployment configuration
func InitializeDeployment(
	ctx context.Context,
	uniqueID string,
	disp *display.Display,
) (*models.Deployment, error) {
	l := logger.Get()
	v := viper.GetViper()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		l.Info("Deployment cancelled before starting")
		return nil, fmt.Errorf("deployment cancelled: %w", err)
	}

	// Set default values for all configuration items
	v.SetDefault("general.project_id", "default-project")
	v.SetDefault("general.log_path", "/var/log/andaime")
	v.SetDefault("general.log_level", "info")
	v.SetDefault("general.ssh_public_key_path", "~/.ssh/id_rsa.pub")
	v.SetDefault("general.ssh_private_key_path", "~/.ssh/id_rsa")
	v.SetDefault("azure.resource_group_name", "andaime-rg")
	v.SetDefault("azure.resource_group_location", "eastus")
	v.SetDefault("azure.allowed_ports", []int{22, 80, 443})
	v.SetDefault("azure.machines", []models.Machine{
		{
			Name:     "default-vm",
			VMSize:   "Standard_B2s",
			Location: "eastus",
			Parameters: []models.Parameter{
				{Orchestrator: true},
			},
		},
	})

	// Extract Azure-specific configuration
	projectID := v.GetString("general.project_id")

	// Create deployment object
	deployment, err := PrepareDeployment(ctx, v, projectID, uniqueID, disp)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func EnsureTags(tags map[string]*string, projectID, uniqueID string) map[string]*string {
	if tags == nil {
		tags = map[string]*string{}
	}
	if tags["andaime"] == nil {
		tags["andaime"] = to.Ptr("true")
	}
	if tags["deployed-by"] == nil {
		tags["deployed-by"] = to.Ptr("andaime")
	}
	if tags["andaime-resource-tracking"] == nil {
		tags["andaime-resource-tracking"] = to.Ptr("true")
	}
	if tags["unique-id"] == nil {
		tags["unique-id"] = to.Ptr(uniqueID)
	}
	if tags["project-id"] == nil {
		tags["project-id"] = to.Ptr(projectID)
	}
	if tags["andaime-project"] == nil {
		tags["andaime-project"] = to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID))
	}
	return tags
}

// prepareDeployment sets up the initial deployment configuration
func PrepareDeployment(
	ctx context.Context,
	viper *viper.Viper,
	projectID, uniqueID string,
	disp *display.Display,
) (*models.Deployment, error) {
	l := logger.Get()

	// Extract SSH keys
	sshPublicKeyPath, sshPrivateKeyPath, err := ExtractSSHKeyPaths()
	if err != nil {
		return nil, err
	}

	// Ensure tags
	tags := EnsureTags(make(map[string]*string), projectID, uniqueID)

	// Validate SSH keys
	if err := sshutils.ValidateSSHKeysFromPath(sshPublicKeyPath, sshPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to validate SSH keys: %v", err)
	}

	// Unmarshal machines configuration
	var machines []models.Machine
	if err := viper.UnmarshalKey("azure.machines", &machines); err != nil {
		return nil, fmt.Errorf("error unmarshaling machines: %v", err)
	}

	// Get resource group location
	resourceGroupLocation := viper.GetString("azure.resource_group_location")
	if resourceGroupLocation == "" {
		return nil, fmt.Errorf("resource group location is empty")
	}

	deployment := &models.Deployment{
		ProjectID:             projectID,
		UniqueID:              uniqueID,
		ResourceGroupLocation: resourceGroupLocation,
		Tags:                  tags,
	}

	// Set ResourceGroupName only if it's not already set
	if deployment.ResourceGroupName == "" {
		resourceGroupName := viper.GetString("azure.resource_group_name")
		if resourceGroupName == "" {
			return nil, fmt.Errorf("resource group name is empty")
		}
		deployment.ResourceGroupName = resourceGroupName
	}

	// Ensure the deployment has a name
	if deployment.Name == "" {
		deployment.Name = fmt.Sprintf("Azure Deployment - %s", uniqueID)
	}

	// Update Viper configuration
	if err := deployment.UpdateViperConfig(); err != nil {
		return nil, fmt.Errorf("failed to update Viper configuration: %v", err)
	}

	// Get allowed ports
	ports := viper.GetIntSlice("azure.allowed_ports")
	if len(ports) == 0 {
		return nil, fmt.Errorf("no allowed ports found in viper config")
	}
	l.Debugf("Allowed ports: %v", ports)

	// Process machines
	orchestratorNode, allMachines, locations, err := ProcessMachinesConfig(
		machines,
		disp,
	)
	if err != nil {
		return nil, err
	}

	deployment.OrchestratorNode = orchestratorNode
	deployment.Machines = allMachines
	deployment.Locations = locations

	return deployment, nil
}

// extractSSHKeys extracts SSH public and private key contents from Viper configuration
func ExtractSSHKeyPaths() (string, string, error) {
	publicKeyPath, err := extractSSHKeyPath("general.ssh_public_key_path")
	if err != nil {
		return "", "", fmt.Errorf("failed to extract public key material: %w", err)
	}

	privateKeyPath, err := extractSSHKeyPath("general.ssh_private_key_path")
	if err != nil {
		return "", "", fmt.Errorf("failed to extract private key material: %w", err)
	}
	return publicKeyPath, privateKeyPath, nil
}

func extractSSHKeyPath(configKeyString string) (string, error) {
	l := logger.Get()

	l.Debugf("Extracting key path from %s", configKeyString)
	keyPath := viper.GetString(configKeyString)
	if keyPath == "" {
		return "", fmt.Errorf(
			"%s is empty. \nConfig key used: %s \nConfig file used: %s",
			configKeyString,
			configKeyString,
			viper.ConfigFileUsed(),
		)
	}

	l.Debugf("Extracting key material from %s", keyPath)
	if keyPath == "" {
		return "", fmt.Errorf("key path is empty")
	}

	l.Debugf("Getting absolute path for key file: %s", keyPath)

	// If path starts with "~" then replace with homedir
	if strings.HasPrefix(keyPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[1:])
	}

	absoluteKeyPath, err := filepath.Abs(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for key file: %w", err)
	}

	l.Debugf("Reading key file: %s", absoluteKeyPath)
	keyMaterial, err := os.ReadFile(absoluteKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	if len(keyMaterial) == 0 {
		return "", fmt.Errorf("key file is empty")
	}

	return absoluteKeyPath, nil
}

// processMachinesConfig processes the machine configurations
func ProcessMachinesConfig(
	machines []models.Machine,
	disp *display.Display,
) (*models.Machine, []models.Machine, []string, error) {
	var orchestratorNode *models.Machine
	var allMachines []models.Machine
	locations := make(map[string]bool)

	for _, machine := range machines {
		internalMachine := machine

		if internalMachine.Location == "" {
			return nil, nil, nil, fmt.Errorf("machine location is empty")
		}

		if !internal.IsValidLocation(internalMachine.Location) {
			return nil, nil, nil, fmt.Errorf("invalid location: %s", internalMachine.Location)
		}
		locations[internalMachine.Location] = true

		internalMachine.ID = utils.CreateShortID()

		if len(machine.Parameters) > 0 && machine.Parameters[0].Orchestrator {
			if orchestratorNode != nil {
				return nil, nil, nil, fmt.Errorf("multiple orchestrator nodes found")
			}
			orchestratorNode = &internalMachine
		}
		allMachines = append(allMachines, internalMachine)

		disp.UpdateStatus(&models.Status{
			ID:       internalMachine.ID,
			Type:     "VM",
			Location: internalMachine.Location,
			Status:   "Initializing",
		})

	}

	uniqueLocations := make([]string, 0, len(locations))
	for location := range locations {
		uniqueLocations = append(uniqueLocations, location)
	}

	return orchestratorNode, allMachines, uniqueLocations, nil
}
