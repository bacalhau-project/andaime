package provision

import (
	"fmt"

	"github.com/spf13/cobra"
)

func runProvision(cmd *cobra.Command, args []string) error {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create new provisioner
	provisioner, err := NewProvisioner(config)
	if err != nil {
		return fmt.Errorf("failed to create provisioner: %w", err)
	}

	// Run provisioning
	if err := provisioner.Provision(cmd.Context()); err != nil {
		return fmt.Errorf("provisioning failed: %w", err)
	}

	return nil
}
