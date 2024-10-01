package azure

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetAzureCreateVMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-vm [resource-group-name]",
		Short: "Create a new VM in Azure",
		Args:  cobra.MaximumNArgs(1),
		RunE:  createVM,
	}

	cmd.Flags().StringP("location", "l", "", "The location where the VM will be created")
	cmd.Flags().StringP("vm-size", "s", "", "The size of the VM")
	_ = cmd.MarkFlagRequired("location")
	_ = cmd.MarkFlagRequired("vm-size")

	return cmd
}

func createVM(cmd *cobra.Command, args []string) error {
	// Create a new Viper instance for this command
	viper := viper.New()

	// Set default values
	viper.SetDefault("general.project_prefix", "andaime-single-vm")
	viper.SetDefault("general.ssh_user", viper.GetString("general.ssh_user"))
	viper.SetDefault(
		"general.ssh_public_key_path",
		viper.GetString("general.ssh_public_key_path"),
	)
	viper.SetDefault(
		"general.ssh_private_key_path",
		viper.GetString("general.ssh_private_key_path"),
	)
	viper.SetDefault("azure.subscription_id", viper.GetString("azure.subscription_id"))

	// Get command line flags
	location, _ := cmd.Flags().GetString("location")
	vmSize, _ := cmd.Flags().GetString("vm-size")

	// Set values from flags
	viper.Set("azure.resource_group_location", location)
	viper.Set("azure.default_machine_type", vmSize)

	// Handle resource group
	var resourceGroupName string
	if len(args) > 0 {
		resourceGroupName = args[0]
		viper.Set("azure.resource_group_name", resourceGroupName)
	} else {
		resourceGroupName = fmt.Sprintf("%s-rg", viper.GetString("general.project_prefix"))
		viper.Set("azure.resource_group_name", resourceGroupName)
	}

	// Call the existing executeCreateDeployment function from create_deployment.go
	if err := ExecuteCreateDeployment(cmd, nil); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	// Print the VM details
	m := display.GetGlobalModelFunc()
	for _, machine := range m.Deployment.GetMachines() {
		fmt.Printf(
			"VM created successfully: %s (External IP: %s)\n",
			machine.GetName(),
			machine.GetPublicIP(),
		)
	}

	return nil
}
