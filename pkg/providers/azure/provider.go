package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	GetClient() AzureClient
	SetClient(client AzureClient)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)

	StartResourcePolling(ctx context.Context)
	DeployResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	DestroyResources(ctx context.Context, resourceGroupName string) error
}

type AzureProvider struct {
	Client     AzureClient
	Config     *viper.Viper
	Deployment *models.Deployment
	SSHUser    string
	SSHPort    int
	lastResourceQuery time.Time
	cachedResources   []interface{}
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

func (p *AzureProvider) GetClient() AzureClient {
	return p.Client
}

func (p *AzureProvider) SetClient(client AzureClient) {
	p.Client = client
}

func (p *AzureProvider) GetConfig() *viper.Viper {
	return p.Config
}

func (p *AzureProvider) SetConfig(config *viper.Viper) {
	p.Config = config
}

func (p *AzureProvider) DestroyResources(ctx context.Context, resourceGroupName string) error {
	return p.Client.DestroyResourceGroup(ctx, resourceGroupName)
}

// Updates the deployment with the latest resource state
func (p *AzureProvider) ListAllResourcesInSubscription(ctx context.Context,
	subscriptionID string,
	tags map[string]*string) ([]interface{}, error) {
	l := logger.Get()

	// Check if we can use cached results
	if time.Since(p.lastResourceQuery) < 30*time.Second {
		l.Debug("Using cached resources")
		return p.cachedResources, nil
	}

	start := time.Now()
	resources, err := p.Client.ListAllResourcesInSubscription(ctx,
		subscriptionID,
		tags)
	l.Debugf("ListAllResourcesInSubscription took %v", time.Since(start))

	if err != nil {
		l.Errorf("Failed to query Azure resources: %v", err)
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
	l.Debug("Starting StartResourcePolling")

	resourceTicker := time.NewTicker(30 * time.Second)
	defer resourceTicker.Stop()

	for {
		select {
		case <-resourceTicker.C:
			start := time.Now()
			if _, err := p.PollAndUpdateResources(ctx); err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
			}
			l.Debugf("PollAndUpdateResources took %v", time.Since(start))
		case <-ctx.Done():
			l.Debug("Context done received, exiting resource polling")
			return
		}
	}
}

func (p *AzureProvider) PollAndUpdateResources(ctx context.Context) (bool, error) {
	l := logger.Get()
	start := time.Now()
	defer func() {
		l.Debugf("PollAndUpdateResources execution took %v", time.Since(start))
	}()

	select {
	case <-ctx.Done():
		l.Debug("Cancel command received in PollAndUpdateResources")
		return false, ctx.Err()
	default:
		// Continue with the function if no cancellation
	}

	// Rest of the function...
}

var _ AzureProviderer = &AzureProvider{}
