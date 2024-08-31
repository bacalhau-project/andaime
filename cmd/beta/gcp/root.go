package gcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var GCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "GCP-related commands",
	Long:  `Commands for interacting with Google Cloud Platform (GCP).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		organizationID := viper.GetString("gcp.organization_id")
		if organizationID == "" {
			return fmt.Errorf("organization_id is not set in the configuration")
		}

		p, err := gcp.NewGCPProviderFunc()
		if err != nil {
			return err
		}

		// Check authentication
		if err := p.CheckAuthentication(ctx); err != nil {
			return handleGCPError(err)
		}

		return nil
	},
}

var initOnce sync.Once

func InitializeCommands() {
	initOnce.Do(func() {
		GCPCmd.AddCommand(GetCreateProjectCmd())
		GCPCmd.AddCommand(GetDestroyProjectCmd())
		GCPCmd.AddCommand(GetListAllAssetsInProjectCmd())
		GCPCmd.AddCommand(GetCreateVMCmd()) // Add this line
		// Add other commands here
	})
}

func GetGCPCmd() *cobra.Command {
	InitializeCommands()
	return GCPCmd
}

func handleGCPError(err error) error {
	if strings.Contains(err.Error(), "Unauthenticated") ||
		strings.Contains(err.Error(), "invalid_grant") {
		return fmt.Errorf(`
Authentication error: It seems your GCP credentials are not set up correctly or have expired.

Please follow these steps to authenticate:

1. Ensure you have the gcloud CLI installed. If not, download it from:
   https://cloud.google.com/sdk/docs/install

2. Run the following commands in your terminal:
   gcloud auth login
   gcloud auth application-default login

3. Follow the prompts to log in to your Google account.

4. Once authenticated, try running the command again.

If you continue to experience issues, please check your organization ID
in the configuration file and ensure you have the necessary permissions.

Original error: %v`, err)
	}
	return err
}
