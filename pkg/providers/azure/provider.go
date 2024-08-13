package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	GetClient() AzureClient
	SetClient(client AzureClient)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)

	StartResourcePolling(ctx context.Context, done chan<- struct{})
	GetResources(ctx context.Context, resourceGroupName string) ([]interface{}, error)
	DeployResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	DestroyResources(ctx context.Context, resourceGroupName string) error
}

type AzureProvider struct {
	Client     AzureClient
	Config     *viper.Viper
	Deployment *models.Deployment
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

	// Check for SSH key
	sshPublicKeyPath := config.GetString("azure.ssh_public_key_path")
	if sshPublicKeyPath == "" {
		return nil, fmt.Errorf("azure.ssh_public_key_path is required")
	}
	if _, err := os.Stat(sshPublicKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SSH public key file does not exist: %s", sshPublicKeyPath)
	}

	subscriptionID := config.GetString("azure.subscription_id")
	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	return &AzureProvider{
		Client:     client,
		Config:     config,
		Deployment: models.NewDeployment(),
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
	tags map[string]*string) error {
	l := logger.Get()

	err := p.Client.ListAllResourcesInSubscription(ctx,
		subscriptionID,
		tags)
	if err != nil {
		l.Errorf("Failed to query Azure resources: %v", err)
		return fmt.Errorf("failed to query resources: %v", err)
	}

	l.Debugf("Azure Resource Graph response - done listing resources.")

	return nil
}

func (p *AzureProvider) StartResourcePolling(ctx context.Context, done chan<- struct{}) {
	l := logger.Get()
	l.Debug("Starting StartResourcePolling")

	resourceTicker := time.NewTicker(1 * time.Second)
	defer resourceTicker.Stop()

	for {
		select {
		case <-resourceTicker.C:
			if err, _ := p.PollAndUpdateResources(ctx); err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
			}
			p.updateStatusMessage()
		case <-ctx.Done():
			l.Debug("Context done, exiting resource polling")
			close(done)
			return
		}
	}
}

func (p *AzureProvider) updateStatusMessage() {
	l := logger.Get()
	totalMachines := len(p.Deployment.Machines)
	runningMachines := 0
	for _, machine := range p.Deployment.Machines {
		if machine.Status == "Succeeded" {
			runningMachines++
		}
	}
	statusMsg := fmt.Sprintf("Machines: %d/%d running", runningMachines, totalMachines)
	l.Infof(statusMsg)
}

func (p *AzureProvider) GetResources(
	ctx context.Context,
	resourceGroupName string,
) ([]interface{}, error) {
	return p.Client.GetResources(ctx, resourceGroupName)
}

// PollAndUpdateResources is now defined in pkg/providers/azure/deploy.go

var _ AzureProviderer = &AzureProvider{}
