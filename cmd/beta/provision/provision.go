package provision

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func runProvision(cmd *cobra.Command, args []string) error {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create new provisioner
	l := logger.GetLogger()
	l.Debug("Creating new provisioner with config")
	l.Debugf("Config details - IP: %s, Username: %s", config.IPAddress, config.Username)
	
	provisioner, err := NewProvisioner(config)
	if err != nil {
		l.Errorf("Failed to create provisioner: %v", err)
		return fmt.Errorf("failed to create provisioner: %w", err)
	}
	l.Debug("Successfully created provisioner")

	// Run provisioning
	l.Debug("Starting provisioning process")
	if err := provisioner.Provision(cmd.Context()); err != nil {
		l.Errorf("Provisioning failed: %v", err)
		return fmt.Errorf("provisioning failed: %w", err)
	}
	l.Info("Provisioning completed successfully")

	return nil
}
