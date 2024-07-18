package azure

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

// DeployResources deploys Azure resources based on the provided configuration
func DeployResources(config *viper.Viper) error {
	// Extract Azure-specific configuration
	subscriptionID := config.GetString("subscription_id")
	projectID := config.GetString("project_id")
	uniqueID := config.GetString("unique_id")
	resourceGroup := config.GetString("resource_group")
	location := config.GetString("location")
	vmName := config.GetString("vm_name")
	vmSize := config.GetString("vm_size")
	diskSizeGB := config.GetInt32("disk_size_gb")

	// Extract SSH public key
	sshPublicKey := config.GetString("ssh_public_key")

	// Extract allowed ports
	allowedPorts := config.GetIntSlice("allowed_ports")

	// Create Azure clients
	clients, err := NewClientInterfaces(subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create Azure clients: %v", err)
	}

	// Deploy VM
	err = DeployVM(context.Background(),
		projectID,
		uniqueID,
		clients,
		resourceGroup,
		location,
		vmName,
		vmSize,
		diskSizeGB,
		allowedPorts,
		sshPublicKey)
	if err != nil {
		return fmt.Errorf("failed to deploy VM: %v", err)
	}

	fmt.Printf("Successfully deployed Azure VM '%s' in resource group '%s'\n", vmName, resourceGroup)
	return nil
}
