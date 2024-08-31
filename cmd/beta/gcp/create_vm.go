package gcp

import (
	"context"
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
			projectID := args[0]
			return createVM(projectID)
		},
	}
	return cmd
}

func createVM(projectID string) error {
	ctx := context.Background()
	p, err := gcp.NewGCPProviderFunc()
	if err != nil {
		return handleGCPError(err)
	}

	// We don't need to pass any VM configuration now, as all values are derived from the config or generated
	vmConfig := make(map[string]string)

	// Create the VM
	vmName, err := p.CreateVM(ctx, projectID, vmConfig)
	if err != nil {
		return handleGCPError(err)
	}

	fmt.Printf("VM created successfully: %s\n", vmName)
	return nil
}
