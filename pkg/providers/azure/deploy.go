package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/utils"
)

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources() error {
	if p.Config == nil {
		return fmt.Errorf("config is nil")
	}

	config := p.Config

	// Extract Azure-specific configuration
	uniqueID := config.GetString("azure.unique_id")
	resourceGroup := config.GetString("azure.resource_group")

	// Extract SSH public key
	sshPublicKey, err := utils.ExpandPath(config.GetString("general.ssh_public_key_path"))
	if err != nil {
		return fmt.Errorf("failed to expand path for SSH public key: %v", err)
	}
	sshPrivateKey, err := utils.ExpandPath(config.GetString("general.ssh_private_key_path"))
	if err != nil {
		return fmt.Errorf("failed to expand path for SSH private key: %v", err)
	}

	if sshPrivateKey == "" {
		// Then we need to extract the private key from the public key
		sshPrivateKey = strings.TrimSuffix(sshPublicKey, ".pub")
	}

	// Validate SSH keys
	err = sshutils.ValidateSSHKeysFromPath(sshPublicKey, sshPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to validate SSH keys: %v", err)
	}

	// Deploy VM
	_, err = DeployVM(
		context.Background(),
		p.Config.GetString("general.project_id"),
		uniqueID,
		p.Client,
		p.Config,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy VM: %v", err)
	}

	fmt.Printf("Successfully deployed Azure VM '%s' in resource group '%s'\n", uniqueID, resourceGroup)
	return nil
}
