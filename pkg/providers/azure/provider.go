package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
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
	Client AzureClient
	Config *viper.Viper
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
		Client: client,
		Config: config,
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

// Updates the state machine with the latest resource state
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

	//nolint:govet,lostcancel // Cancel still works
	resourceCtx, resourceCancel := context.WithCancel(ctx)

	resourceDone := make(chan struct{})

	go p.runResourceTicker(resourceCtx, resourceDone)

	// Directly check if the context is done
	if ctx.Err() != nil {
		resourceCancel()
	}

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
	resourceTicker := time.NewTicker(globals.NumberOfSecondsToProbeResourceGroup * time.Second)
	defer resourceTicker.Stop()
	defer close(done)

	for {
		select {
		case <-resourceTicker.C:
			resources, err := p.PollAndUpdateResources(ctx)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
			}
			_ = resources // TODO: Figure out if this is still necessary
		case <-ctx.Done():
			return
		}
	}
}

func (p *AzureProvider) updateStatus() {
	l := logger.Get()
	allMachinesComplete := true
	dep := GetGlobalDeployment()
	for _, machine := range dep.Machines {
		if machine.Status != models.MachineStatusComplete {
			allMachinesComplete = false
		}
		if machine.Status == models.MachineStatusComplete {
			continue
		}
		display.UpdateStatus(&models.Status{
			ID: machine.Name,
			ElapsedTime: time.Duration(
				time.Since(machine.StartTime).
					Milliseconds() /
					1000, //nolint:gomnd // Divide by 1000 to convert milliseconds to seconds
			),
		})
	}
	if allMachinesComplete {
		l.Debug("All machines complete, resource polling will stop")
	}
}

var _ AzureProviderer = &AzureProvider{}
