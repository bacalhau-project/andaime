package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources() error {
	if p.Config == nil {
		return fmt.Errorf("config is nil")
	}

	config := p.Config

	// Extract Azure-specific configuration
	projectID := config.GetString("azure.project_id")
	uniqueID := config.GetString("azure.unique_id")
	resourceGroup := config.GetString("azure.resource_group")
	location := config.GetString("azure.location")
	vmName := config.GetString("azure.vm_name")
	vmSize := config.GetString("azure.vm_size")
	diskSizeGB := config.GetInt32("azure.disk_size_gb")

	// Extract SSH public key
	sshPublicKey := config.GetString("general.ssh_public_key")
	sshPrivateKey := config.GetString("general.ssh_private_key")

	if sshPrivateKey == "" {
		// Then we need to extract the private key from the public key
		sshPrivateKey = strings.TrimSuffix(sshPublicKey, ".pub")
	}

	// Validate SSH keys
	err := sshutils.ValidateSSHKeysFromPath(sshPublicKey, sshPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to validate SSH keys: %v", err)
	}

	// Extract allowed ports
	allowedPorts := config.GetIntSlice("allowed_ports")

	// Create Azure clients
	client := p.Client

	// Deploy VM
	err = DeployVM(context.Background(),
		projectID,
		uniqueID,
		client,
		resourceGroup,
		location,
		vmName,
		vmSize,
		diskSizeGB,
		allowedPorts,
		sshPublicKey,
		sshPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to deploy VM: %v", err)
	}

	fmt.Printf("Successfully deployed Azure VM '%s' in resource group '%s'\n", vmName, resourceGroup)
	return nil
}
