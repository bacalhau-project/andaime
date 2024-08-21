package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/general"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	general.Providerer
	GetAzureClient() AzureClient
	SetAzureClient(client AzureClient)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)

	StartResourcePolling(ctx context.Context)
	DeployResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	DestroyResources(ctx context.Context, resourceGroupName string) error

	DeployBacalhauOrchestrator(ctx context.Context) error
	DeployBacalhauWorkers(ctx context.Context) error
}

type AzureProvider struct {
	Client            interface{}
	Config            *viper.Viper
	Deployment        *models.Deployment
	SSHUser           string
	SSHPort           int
	lastResourceQuery time.Time
	cachedResources   []interface{}
	goroutineCounter  int64
}

var AzureProviderFunc = NewAzureProvider

// NewAzureProvider creates a new AzureProvider instance
func NewAzureProvider() (AzureProviderer, error) {
	config := viper.GetViper()
	if !config.IsSet("azure") {
		return nil, fmt.Errorf("azure configuration is required")
	}

	if !config.IsSet("azure.subscription_id") {
		return nil, fmt.Errorf("azure.subscription_id is required")
	}

	// Check for SSH keys
	sshPublicKeyPath := config.GetString("general.ssh_public_key_path")
	sshPrivateKeyPath := config.GetString("general.ssh_private_key_path")
	if sshPublicKeyPath == "" {
		return nil, fmt.Errorf("general.ssh_public_key_path is required")
	}
	if sshPrivateKeyPath == "" {
		return nil, fmt.Errorf("general.ssh_private_key_path is required")
	}

	// Expand the paths
	expandedPublicKeyPath, err := homedir.Expand(sshPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand public key path: %w", err)
	}
	expandedPrivateKeyPath, err := homedir.Expand(sshPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand private key path: %w", err)
	}

	// Check if the files exist
	if _, err := os.Stat(expandedPublicKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SSH public key file does not exist: %s", expandedPublicKeyPath)
	}
	if _, err := os.Stat(expandedPrivateKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SSH private key file does not exist: %s", expandedPrivateKeyPath)
	}

	// Update the config with the expanded paths
	config.Set("general.ssh_public_key_path", expandedPublicKeyPath)
	config.Set("general.ssh_private_key_path", expandedPrivateKeyPath)

	subscriptionID := config.GetString("azure.subscription_id")
	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	sshUser := config.GetString("azure.ssh_user")
	if sshUser == "" {
		sshUser = "azureuser" // Default SSH user for Azure VMs
	}

	sshPort := config.GetInt("azure.ssh_port")
	if sshPort == 0 {
		sshPort = 22 // Default SSH port
	}

	return &AzureProvider{
		Client:     client,
		Config:     config,
		Deployment: models.NewDeployment(),
		SSHUser:    sshUser,
		SSHPort:    sshPort,
	}, nil
}

func (p *AzureProvider) GetClient() interface{} {
	return p.Client
}

func (p *AzureProvider) SetClient(client interface{}) {
	p.Client = client
}

func (p *AzureProvider) GetAzureClient() AzureClient {
	return p.Client.(AzureClient)
}

func (p *AzureProvider) SetAzureClient(client AzureClient) {
	p.Client = client
}

func (p *AzureProvider) GetConfig() *viper.Viper {
	return p.Config
}

func (p *AzureProvider) SetConfig(config *viper.Viper) {
	p.Config = config
}

func (p *AzureProvider) DestroyResources(ctx context.Context, resourceGroupName string) error {
	client := p.GetAzureClient()
	return client.DestroyResourceGroup(ctx, resourceGroupName)
}

// Updates the deployment with the latest resource state
func (p *AzureProvider) ListAllResourcesInSubscription(ctx context.Context,
	subscriptionID string,
	tags map[string]*string) ([]interface{}, error) {
	l := logger.Get()
	client := p.GetAzureClient()

	// Check if we can use cached results
	if time.Since(p.lastResourceQuery) < 30*time.Second {
		l.Debug("Using cached resources")
		return p.cachedResources, nil
	}

	start := time.Now()
	resources, err := client.ListAllResourcesInSubscription(ctx,
		subscriptionID,
		tags)
	elapsed := time.Since(start)
	l.Debugf("ListAllResourcesInSubscription took %v", elapsed)
	writeToDebugLog(fmt.Sprintf("ListAllResourcesInSubscription took %v", elapsed))

	if err != nil {
		l.Errorf("Failed to query Azure resources: %v", err)
		writeToDebugLog(fmt.Sprintf("Failed to query Azure resources: %v", err))
		return nil, fmt.Errorf("failed to query resources: %v", err)
	}

	if resources == nil {
		resources = []interface{}{}
	}

	// Update cache
	p.cachedResources = resources
	p.lastResourceQuery = time.Now()

	return resources, nil
}

func (p *AzureProvider) StartResourcePolling(ctx context.Context) {
	l := logger.Get()
	writeToDebugLog("Starting StartResourcePolling")

	resourceTicker := time.NewTicker(5 * time.Second)
	defer resourceTicker.Stop()

	quit := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(quit)
	}()

	done := make(chan bool)
	go func() {
		pollCount := 0
		for {
			select {
			case <-resourceTicker.C:
				m := display.GetGlobalModelFunc()
				if m.Quitting {
					writeToDebugLog("Quitting detected, stopping resource polling")
					done <- true
					return
				}
				pollCount++
				start := time.Now()
				writeToDebugLog(fmt.Sprintf("Starting poll #%d", pollCount))

				resources, err := p.PollAndUpdateResources(ctx)
				if err != nil {
					l.Errorf("Failed to poll and update resources: %v", err)
					writeToDebugLog(fmt.Sprintf("Failed to poll and update resources: %v", err))
					done <- true
					return
				}

				// Check for Bacalhau node listing failure
				if p.checkBacalhauNodeListingFailure() {
					l.Error("Persistent failure in listing Bacalhau nodes detected")
					writeToDebugLog("Persistent failure in listing Bacalhau nodes detected")
					done <- true
					return
				}

				writeToDebugLog(
					fmt.Sprintf("Poll #%d: Found %d resources", pollCount, len(resources)),
				)

				allResourcesProvisioned := true
				for _, resource := range resources {
					resourceMap := resource.(map[string]interface{})
					provisioningState := resourceMap["provisioningState"].(string)
					writeToDebugLog(
						fmt.Sprintf(
							"Resource: %s - Provisioning State: %s",
							resourceMap["name"].(string),
							provisioningState,
						),
					)
					if provisioningState != "Succeeded" {
						allResourcesProvisioned = false
					}
				}

				elapsed := time.Since(start)
				writeToDebugLog(
					fmt.Sprintf("PollAndUpdateResources #%d took %v", pollCount, elapsed),
				)

				p.logDeploymentStatus()

				if allResourcesProvisioned {
					writeToDebugLog("All resources provisioned, stopping resource polling")
					done <- true
					return
				}

			case <-quit:
				l.Debug("Quit signal received, exiting resource polling")
				writeToDebugLog("Quit signal received, exiting resource polling")
				done <- true
				return
			}
		}
	}()

	select {
	case <-quit:
		l.Debug("Quit signal received, forcing immediate exit")
		writeToDebugLog("Quit signal received, forcing immediate exit")
	case <-done:
		l.Debug("Resource polling completed")
		writeToDebugLog("Resource polling completed")
	}

	utils.CloseAllChannels()
}

func (p *AzureProvider) logDeploymentStatus() {
	if p.Deployment == nil {
		writeToDebugLog("Deployment is nil")
		return
	}

	writeToDebugLog(
		fmt.Sprintf(
			"Deployment Status - Name: %s, ResourceGroup: %s",
			p.Deployment.Name,
			p.Deployment.ResourceGroupName,
		),
	)
	writeToDebugLog(fmt.Sprintf("Total Machines: %d", len(p.Deployment.Machines)))

	for _, machine := range p.Deployment.Machines {
		writeToDebugLog(
			fmt.Sprintf(
				"Machine Name: %s, PublicIP: %s, PrivateIP: %s",
				machine.Name,
				machine.PublicIP,
				machine.PrivateIP,
			),
		)
		writeToDebugLog(
			fmt.Sprintf(
				"Machine %s - Docker: %v, CorePackages: %v, Bacalhau: %v, SSH: %v",
				machine.Name,
				machine.GetServiceState("Docker"),
				machine.GetServiceState("CorePackages"),
				machine.GetServiceState("Bacalhau"),
				machine.GetServiceState("SSH"),
			),
		)

		completedResources, totalResources := machine.ResourcesComplete()
		writeToDebugLog(
			fmt.Sprintf(
				"Machine %s - Resources: %d/%d complete",
				machine.Name,
				completedResources,
				totalResources,
			),
		)

		if machine.GetMachineResources() != nil {
			for resourceType, resource := range machine.GetMachineResources() {
				writeToDebugLog(
					fmt.Sprintf(
						"Machine %s - Resource %s: State: %v, Value: %s",
						machine.Name,
						resourceType,
						resource.ResourceState,
						resource.ResourceValue,
					),
				)
			}
		} else {
			writeToDebugLog(fmt.Sprintf("Machine %s - No machine resources", machine.Name))
		}
	}
}

var _ AzureProviderer = &AzureProvider{}

func writeToDebugLog(message string) {
	debugFilePath := "/tmp/andaime-debug.log"
	debugFile, err := os.OpenFile(debugFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening debug log file %s: %v\n", debugFilePath, err)
		return
	}
	defer debugFile.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s\n", timestamp, message)
	if _, err := debugFile.WriteString(logMessage); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to debug log file: %v\n", err)
	}
}

func (p *AzureProvider) checkBacalhauNodeListingFailure() bool {
	// This is a placeholder implementation. You should replace this with actual logic
	// to check if there have been persistent failures in listing Bacalhau nodes.
	// For example, you could keep a counter of consecutive failures and return true
	// if it exceeds a certain threshold.
	
	// For now, we'll just return false to avoid breaking existing functionality.
	return false
}
