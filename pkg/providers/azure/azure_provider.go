package azure

import (
	"context"
	"fmt"
	"os"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var NewAzureProviderFunc = NewAzureProvider

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
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure.subscription_id is empty or not set in the configuration")
	}
	l := logger.Get()
	l.Debugf("Using Azure subscription ID: %s", subscriptionID)

	// Validate the subscription ID format
	if !utils.IsValidGUID(subscriptionID) {
		return nil, fmt.Errorf("invalid Azure subscription ID format: %s", subscriptionID)
	}

	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	sshUser := config.GetString("general.ssh_user")
	if sshUser == "" {
		sshUser = "azureuser" // Default SSH user for Azure VMs
	}

	sshPort := config.GetInt("general.ssh_port")
	if sshPort == 0 {
		sshPort = 22 // Default SSH port
	}

	provider := &AzureProvider{
		Client:      client,
		Config:      config,
		SSHUser:     sshUser,
		SSHPort:     sshPort,
		updateQueue: make(chan UpdateAction, UpdateQueueSize),
	}

	provider.ClusterDeployer = common.NewClusterDeployer(provider)

	go provider.startUpdateProcessor(context.Background())

	// Initialize the display model with machines from the configuration
	provider.initializeDisplayModel()

	return provider, nil
}
