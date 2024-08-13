package azure

import (
	"context"
	"fmt"
	"strings"
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

	resourceCtx, resourceCancel := context.WithCancel(ctx)
	defer resourceCancel()

	resourceDone := make(chan struct{})

	go p.runResourceTicker(resourceCtx, resourceDone)

	<-resourceDone

	l.Debug("Context done, exiting resource polling")
	close(done)
}

func (p *AzureProvider) GetResources(
	ctx context.Context,
	resourceGroupName string,
) ([]interface{}, error) {
	return p.Client.GetResources(ctx, resourceGroupName)
}

func (p *AzureProvider) runResourceTicker(ctx context.Context, done chan<- struct{}) {
	l := logger.Get()
	resourceTicker := time.NewTicker(1 * time.Second)
	defer resourceTicker.Stop()
	defer close(done)

	for {
		select {
		case <-resourceTicker.C:
			if err := p.PollAndUpdateResources(ctx); err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *AzureProvider) PollAndUpdateResources(ctx context.Context) error {
	l := logger.Get()

	resources, err := p.Client.GetResources(ctx, p.Deployment.ResourceGroupName)
	if err != nil {
		return err
	}

	for _, resource := range resources {
		r := resource.(map[string]interface{})
		name := r["name"].(string)
		resourceType := r["type"].(string)
		provisioningState := r["properties"].(map[string]interface{})["provisioningState"].(string)

		status := &models.Status{
			ID:     name,
			Type:   models.UpdateStatusResourceType(resourceType),
			Status: provisioningState,
		}

		switch resourceType {
		case "Microsoft.Network/networkSecurityGroups", "Microsoft.Network/securityRules":
			for _, machine := range p.Deployment.Machines {
				if strings.HasPrefix(name, machine.Location) {
					status.ID = machine.Name
					p.Deployment.UpdateStatus(status)
				}
			}
		case "Microsoft.Network/publicIPAddresses", "Microsoft.Compute/disks", "Microsoft.Network/networkInterfaces":
			for i, machine := range p.Deployment.Machines {
				if strings.HasPrefix(name, machine.Name) {
					status.ID = machine.Name
					p.Deployment.UpdateStatus(status)
					if resourceType == "Microsoft.Network/publicIPAddresses" && provisioningState == "Succeeded" {
						publicIP, err := p.Client.GetPublicIPAddress(ctx, p.Deployment.ResourceGroupName, name)
						if err == nil {
							p.Deployment.Machines[i].PublicIP = *publicIP.IPAddress
						} else {
							l.Errorf("Failed to get public IP address: %v", err)
						}
					}
				}
			}
		default:
			p.Deployment.UpdateStatus(status)
		}
	}

	return nil
}

var _ AzureProviderer = &AzureProvider{}
