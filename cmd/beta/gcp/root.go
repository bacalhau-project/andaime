package gcp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var (
	commandlineProvidedProjectID string
)

var GCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "GCP-related commands",
	Long:  `Commands for interacting with Google Cloud Platform (GCP).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var initOnce sync.Once

func InitializeCommands() {
	initOnce.Do(func() {
		GCPCmd.PersistentFlags().
			StringVar(&commandlineProvidedProjectID, "project-id", "", "GCP project ID (optional)")
		GCPCmd.AddCommand(GetGCPCreateDeploymentCmd())
		GCPCmd.AddCommand(GetGCPCreateProjectCmd())
		GCPCmd.AddCommand(GetGCPCreateVMCmd())
		GCPCmd.AddCommand(GetGCPDestroyCmd())
	})
}

func GetGCPCmd() *cobra.Command {
	InitializeCommands()
	return GCPCmd
}

func handleGCPError(err error) error {
	if strings.Contains(err.Error(), "Unauthenticated") ||
		strings.Contains(err.Error(), "invalid_grant") ||
		strings.Contains(err.Error(), "oauth2: cannot fetch token") {
		fmt.Println(
			`Authentication error: It seems your GCP credentials are not set up correctly or have expired.

Please follow these steps to authenticate:

1. Ensure you have the gcloud CLI installed. If not, download it from:
   https://cloud.google.com/sdk/docs/install

2. You can authenticate using one of the following methods:

   a) Run the following commands in your terminal:
      gcloud auth login
      gcloud auth application-default login

   b) Set the following environment variables:
      export GCP_SERVICE_ACCOUNT_EMAIL=your-service-account-email
      export GCP_SERVICE_ACCOUNT_KEY=your-service-account-key

3. Follow the prompts to log in to your Google account.

4. Once authenticated, try running the command again.

If you continue to experience issues, please check your organization ID
in the configuration file and ensure you have the necessary permissions.`,
		)
		return nil
	}
	return err
}
