package gcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetCreateVMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-vm <project-id>",
		Short: "Create a new VM in a GCP project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := createVM(cmd, args)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			return nil
		},
	}

	cmd.Flags().StringP("zone", "z", "", "The zone where the VM will be created")
	_ = cmd.MarkFlagRequired("zone")

	return cmd
}

// This is only for doing one offs - the create-deployment will not use this function.
func createVM(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("project ID is required")
	}
	projectID := args[0]

	zone, err := cmd.Flags().GetString("zone")
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	provider, err := gcp.NewGCPProvider()
	if err != nil {
		return err
	}

	sshUser := viper.GetString("general.ssh_user")
	if sshUser == "" {
		return fmt.Errorf("ssh user is not set")
	}

	publicKeyPath := viper.GetString("general.ssh_public_key_path")
	publicKeyMaterial, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return err
	}

	if publicKeyMaterial == nil {
		return fmt.Errorf("public key material is nil, read from file %s", publicKeyPath)
	}

	vmConfig := map[string]string{
		"zone":              zone,
		"sshUser":           sshUser,
		"PublicKeyMaterial": string(publicKeyMaterial),
	}

	vmName, err := provider.CreateVM(ctx, projectID, vmConfig)
	if err != nil {
		if strings.Contains(err.Error(), "Unknown zone") {
			return fmt.Errorf(
				"invalid zone '%s'. Please use a valid zone. You can list all available zones using the following command:\n\tgcloud compute zones list",
				zone,
			)
		}
		return err
	}

	// Get the external IP address of the VM
	externalIP, err := provider.GetVMExternalIP(ctx, projectID, zone, vmName)
	if err != nil {
		return err
	}

	fmt.Printf("VM created successfully: %s (External IP: %s)\n", vmName, externalIP)
	return nil
}
