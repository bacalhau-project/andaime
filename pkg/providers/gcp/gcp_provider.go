package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/viper"
)

// NewGCPProviderFunc is a function type that creates a new GCPProvider
type NewGCPProviderFunc func(client GCPClienter) *GCPProvider

// DefaultNewGCPProvider is the default implementation of NewGCPProviderFunc
var DefaultNewGCPProvider NewGCPProviderFunc = NewGCPProvider

type GCPProvider struct {
	Client          GCPClienter
	Config          *viper.Viper
	updateQueue     chan UpdateAction
	ClusterDeployer *common.ClusterDeployer
}

func NewGCPProvider(client GCPClienter) *GCPProvider {
	return &GCPProvider{
		Client:          client,
		Config:          viper.GetViper(),
		ClusterDeployer: common.NewClusterDeployer(),
	}
}

func (p *GCPProvider) Initialize(ctx context.Context) error {
	p.updateQueue = make(chan UpdateAction, UpdateQueueSize)
	go p.startUpdateProcessor(ctx)
	return nil
}

func (p *GCPProvider) GetOrCreateResourceGroup(ctx context.Context) (*models.ResourceGroup, error) {
	projectID := p.Config.GetString("gcp.project_id")
	location := p.Config.GetString("gcp.location")

	project, err := p.Client.EnsureProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure project: %w", err)
	}

	return &models.ResourceGroup{
		Name:     project.Name,
		Location: location,
		ID:       project.ProjectId,
	}, nil
}

func (p *GCPProvider) DestroyResourceGroup(ctx context.Context) error {
	projectID := p.Config.GetString("gcp.project_id")
	return p.Client.DestroyProject(ctx, projectID)
}

func (p *GCPProvider) CreateVM(ctx context.Context, machine *models.Machine) error {
	// Implementation for creating VM using GCPClient
	return nil
}

func (p *GCPProvider) DeleteVM(ctx context.Context, machine *models.Machine) error {
	// Implementation for deleting VM using GCPClient
	return nil
}

func (p *GCPProvider) GetVMExternalIP(
	ctx context.Context,
	machine *models.Machine,
) (string, error) {
	projectID := p.Config.GetString("gcp.project_id")
	zone := p.Config.GetString("gcp.zone")
	return p.Client.GetVMExternalIP(ctx, projectID, zone, machine.Name)
}

func (p *GCPProvider) SetupNetworking(ctx context.Context) error {
	// Implementation for networking setup
	return nil
}

func (p *GCPProvider) ConfigureFirewall(ctx context.Context, machine *models.Machine) error {
	return p.Client.ConfigureFirewall(ctx, machine)
}

func (p *GCPProvider) ValidateMachineType(ctx context.Context, machineType string) (bool, error) {
	zone := p.Config.GetString("gcp.zone")
	return p.Client.ValidateMachineType(ctx, machineType, zone)
}

func (p *GCPProvider) SetBillingAccount(ctx context.Context, accountID string) error {
	projectID := p.Config.GetString("gcp.project_id")
	return p.Client.SetBillingAccount(ctx, projectID, accountID)
}

func (p *GCPProvider) FinalizeDeployment(ctx context.Context) error {
	// Finalization logic
	return nil
}

func (p *GCPProvider) GetClusterDeployer() *common.ClusterDeployer {
	return p.ClusterDeployer
}

// Ensure GCPProvider implements the Provider interface
var _ providers.Providerer = &GCPProvider{}

func (p *GCPProvider) startUpdateProcessor(ctx context.Context) {
	// Implementation remains the same
}

func (p *GCPProvider) processUpdate(update UpdateAction) {
	// Implementation remains the same
}
