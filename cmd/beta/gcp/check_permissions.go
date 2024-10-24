package gcp

import (
	"github.com/bacalhau-project/andaime/pkg/logger"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
)

var checkPermissionsCmd = &cobra.Command{
	Use:   "check-permissions",
	Short: "Check GCP permissions",
	Long:  `Check the current user's GCP permissions required for Andaime.`,
	RunE:  executeCheckPermissions,
}

func GetGCPCheckPermissionsCmd() *cobra.Command {
	return checkPermissionsCmd
}

func executeCheckPermissions(cmd *cobra.Command, _ []string) error {
	l := logger.Get()
	ctx := cmd.Context()

	organizationID := cmd.Flag("organization-id").Value.String()
	billingAccountID := cmd.Flag("billing-account-id").Value.String()

	gcpProvider, err := gcp_provider.NewGCPProviderFunc(ctx, organizationID, billingAccountID)
	if err != nil {
		return err
	}

	l.Info("Checking GCP permissions...")
	err = gcpProvider.CheckPermissions(ctx)
	if err != nil {
		l.Errorf("Permission check failed: %v", err)
		return err
	}

	l.Info("All required permissions are granted.")
	return nil
}
