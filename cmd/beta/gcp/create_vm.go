package gcp

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
)

func GetCreateVMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-vm <project-id>",
		Short: "Create a new VM in a GCP project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return createVM(cmd, args)
		},
	}
	return cmd
}

func createVM(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("project ID is required")
	}
	projectID := args[0]

	ctx := cmd.Context()
	provider, err := gcp.NewGCPProvider()
	if err != nil {
		return err
	}

	vmConfig := map[string]string{
		// Add any VM configuration parameters here
	}

	vmName, err := provider.CreateVM(ctx, projectID, vmConfig)
	if err != nil {
		return err
	}

	fmt.Printf("VM created successfully: %s\n", vmName)
	return nil
}
