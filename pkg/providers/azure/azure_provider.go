package azure

import (
	"context"
	"fmt"
	"os"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type AzureProvider struct {
	Client          AzureClienter
	Config          *viper.Viper
	ClusterDeployer *common.ClusterDeployer
	SSHUser         string
	SSHPort         int
	updateQueue     chan common.UpdateAction
}

func NewAzureProvider(ctx context.Context) (providers.Providerer, error) {
	subscriptionID := viper.GetString("azure.subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure.subscription_id is required")
	}

	client, err := NewAzureClient(
		subscriptionID,
	) // Implement this function to create an Azure client
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}
	provider := &AzureProvider{
		Client:          client,
		Config:          viper.GetViper(),
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAzure),
	}

	if err := provider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	return provider, nil
}

// Ensure AzureProvider implements the Providerer interface
var _ providers.Providerer = &AzureProvider{}

func (p *AzureProvider) Initialize(ctx context.Context) error {
	// Check for SSH keys
	sshPublicKeyPath := p.Config.GetString("general.ssh_public_key_path")
	sshPrivateKeyPath := p.Config.GetString("general.ssh_private_key_path")
	if sshPublicKeyPath == "" {
		return fmt.Errorf("general.ssh_public_key_path is required")
	}
	if sshPrivateKeyPath == "" {
		return fmt.Errorf("general.ssh_private_key_path is required")
	}

	// Expand the paths
	expandedPublicKeyPath, err := homedir.Expand(sshPublicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand public key path: %w", err)
	}
	expandedPrivateKeyPath, err := homedir.Expand(sshPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand private key path: %w", err)
	}

	// Check if the files exist
	if _, err := os.Stat(expandedPublicKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH public key file does not exist: %s", expandedPublicKeyPath)
	}
	if _, err := os.Stat(expandedPrivateKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH private key file does not exist: %s", expandedPrivateKeyPath)
	}

	// Update the config with the expanded paths
	p.Config.Set("general.ssh_public_key_path", expandedPublicKeyPath)
	p.Config.Set("general.ssh_private_key_path", expandedPrivateKeyPath)

	p.SSHUser = p.Config.GetString("general.ssh_user")
	if p.SSHUser == "" {
		p.SSHUser = "azureuser" // Default SSH user for Azure VMs
	}

	p.SSHPort = p.Config.GetInt("general.ssh_port")
	if p.SSHPort == 0 {
		p.SSHPort = 22 // Default SSH port
	}

	p.updateQueue = make(chan UpdateAction, UpdateQueueSize)
	go p.startUpdateProcessor(ctx)

	return nil
}

func (p *AzureProvider) GetOrCreateResourceGroup(
	ctx context.Context,
) (*models.ResourceGroup, error) {
	resourceGroupName := p.Config.GetString("azure.resource_group_name")
	location := p.Config.GetString("azure.location")

	resourceGroup, err := p.Client.GetResourceGroup(ctx, resourceGroupName, location)
	if err != nil {
		if azureErr, ok := err.(AzureError); ok && azureErr.IsNotFound() {
			resourceGroup, err = p.Client.GetOrCreateResourceGroup(
				ctx,
				resourceGroupName,
				location,
				nil,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create resource group: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get resource group: %w", err)
		}
	}

	return &models.ResourceGroup{
		Name:     resourceGroup.Name,
		Location: *resourceGroup.Location,
		ID:       *resourceGroup.ID,
	}, nil
}

func (p *AzureProvider) DestroyResourceGroup(ctx context.Context) error {
	resourceGroupName := p.Config.GetString("azure.resource_group_name")
	return p.Client.DestroyResourceGroup(ctx, resourceGroupName)
}

func (p *AzureProvider) CreateVM(ctx context.Context, machine *models.Machine) error {
	// Implementation for creating VM using AzureClient
	return nil
}

func (p *AzureProvider) DeleteVM(ctx context.Context, machine *models.Machine) error {
	// Implementation for deleting VM using AzureClient
	return nil
}

func (p *AzureProvider) SetupNetworking(ctx context.Context) error {
	// Implementation for networking setup
	return nil
}

func (p *AzureProvider) ConfigureFirewall(ctx context.Context, machine *models.Machine) error {
	// Implementation for firewall configuration
	return nil
}

func (p *AzureProvider) ValidateMachineType(ctx context.Context, machineType string) (bool, error) {
	location := p.Config.GetString("azure.location")
	return p.Client.ValidateMachineType(ctx, location, machineType)
}

func (p *AzureProvider) SetBillingAccount(ctx context.Context, accountID string) error {
	l := logger.Get()
	l.Warnf("AzureProvider.SetBillingAccount is not implemented (should not be called)")
	return nil
}

// finalizeDeployment performs any necessary cleanup and final steps
func (p *AzureProvider) FinalizeDeployment(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	goRoutineID := m.RegisterGoroutine(
		fmt.Sprintf("FinalizeDeployment-%s", m.Deployment.Azure.ResourceGroupName),
	)
	defer m.DeregisterGoroutine(goRoutineID)

	l := logger.Get()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		l.Info("Deployment cancelled during finalization")
		return fmt.Errorf("deployment cancelled: %w", err)
	}

	// Log successful completion
	l.Info("Azure deployment completed successfully")

	// Ensure all configurations are saved
	if err := m.Deployment.UpdateViperConfig(); err != nil {
		l.Errorf("Failed to save final configuration: %v", err)
		return fmt.Errorf("failed to save final configuration: %w", err)
	}

	l.Info("Deployment finalized successfully")

	return nil
}

// Ensure AzureProvider implements the Provider interface
var _ providers.Provider = &AzureProvider{}
