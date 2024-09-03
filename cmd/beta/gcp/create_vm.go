package gcp

import (
	"fmt"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
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

	vmConfig := map[string]string{
		"zone": zone,
		// Add any VM configuration parameters here
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
