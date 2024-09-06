package gcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/google"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
)

var (
	commandlineProvidedProjectID string
)

var GCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "GCP-related commands",
	Long:  `Commands for interacting with Google Cloud Platform (GCP).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Check both individual and application default credentials
		if err := checkGCPAuthentication(ctx); err != nil {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			return handleGCPError(err)
		}

		organizationID := viper.GetString("gcp.organization_id")
		if organizationID == "" {
			fmt.Printf(
				"'organization_id' is not set in the configuration file: %s\n",
				viper.ConfigFileUsed(),
			)
			return nil
		}

		m := display.GetGlobalModelFunc()
		if !viper.IsSet("gcp.billing_account_id") {
			fmt.Printf(
				"'gcp.billing_account_id' is not set in the configuration file: %s\n",
				viper.ConfigFileUsed(),
			)
			return nil
		}
		billingAccountID := viper.GetString("gcp.billing_account_id")
		m.Deployment.GCP.BillingAccountID = billingAccountID

		// Check required APIs on the admin project (what this is executing on)
		if err := checkRequiredAPIs(ctx, organizationID); err != nil {
			return err
		}

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
		GCPCmd.AddCommand(GetGCPDestroyProjectCmd())
	})
}

func GetGCPCmd() *cobra.Command {
	InitializeCommands()
	return GCPCmd
}

func checkGCPAuthentication(ctx context.Context) error {
	// Check if service account credentials are provided via environment variables
	serviceAccountEmail := os.Getenv("GCP_SERVICE_ACCOUNT_EMAIL")
	serviceAccountKey := os.Getenv("GCP_SERVICE_ACCOUNT_KEY")

	if serviceAccountEmail != "" && serviceAccountKey != "" {
		// Use service account credentials
		creds, err := google.CredentialsFromJSON(
			ctx,
			[]byte(serviceAccountKey),
			cloudresourcemanager.CloudPlatformScope,
		)
		if err != nil {
			return fmt.Errorf("failed to create credentials from service account key: %v", err)
		}

		crmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentials(creds))
		if err != nil {
			return fmt.Errorf("failed to create Cloud Resource Manager service: %v", err)
		}

		_, err = crmService.Projects.List().Do()
		if err != nil {
			return fmt.Errorf(
				"failed to list projects (service account credentials may be invalid): %v",
				err,
			)
		}
	} else {
		// Fall back to application default credentials
		_, err := google.FindDefaultCredentials(ctx, cloudresourcemanager.CloudPlatformScope)
		if err != nil {
			return fmt.Errorf("application default credentials check failed: %v", err)
		}

		crmService, err := cloudresourcemanager.NewService(ctx)
		if err != nil {
			return fmt.Errorf("failed to create Cloud Resource Manager service: %v", err)
		}

		_, err = crmService.Projects.List().Do()
		if err != nil {
			return fmt.Errorf("failed to list projects (individual credentials may be invalid): %v", err)
		}
	}

	return nil
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

func checkRequiredAPIs(ctx context.Context, organizationID string) error {
	requiredAPIs := []string{
		"compute.googleapis.com",
		"cloudresourcemanager.googleapis.com",
		"iam.googleapis.com",
		"serviceusage.googleapis.com",
		"cloudbilling.googleapis.com",
		"networkmanagement.googleapis.com",
		"storage-api.googleapis.com",
		"file.googleapis.com",
		"storage.googleapis.com",
		"cloudasset.googleapis.com",
	}

	var adminProjectID string

	if commandlineProvidedProjectID == "" {

		// Test to make sure we can see gcloud binary
		cmd := exec.Command("gcloud", "version")
		_, err := cmd.Output()
		if err != nil {
			fmt.Println(
				"Failed to execute gcloud command - is it installed and on your path?",
			)
			return fmt.Errorf("failed to execute gcloud command")
		}

		// Execute the 'gcloud config get-value project' command
		cmd = exec.Command("gcloud", "config", "get-value", "project")
		projectIDRaw, err := cmd.Output()
		if err != nil {
			fmt.Printf(
				"Failed to get project ID from gcloud command: %v\n",
				err,
			)
			return fmt.Errorf("failed to get project ID from gcloud command")
		}

		// Trim any leading/trailing whitespace and newlines from the output
		adminProjectID = strings.TrimSpace(string(projectIDRaw))
		if adminProjectID == "" {
			return fmt.Errorf("failed to get project ID from gcloud command")
		}
		fmt.Printf("Using admin project ID: %s\n", adminProjectID)
	} else {
		adminProjectID = commandlineProvidedProjectID
	}
	gcpClient, cleanup, err := gcp.NewGCPClient(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("failed to create GCP client: %v", err)
	}
	defer cleanup()

	disabledAPIs := []string{}

	for _, api := range requiredAPIs {
		enabled, err := gcpClient.IsAPIEnabled(ctx, adminProjectID, api)
		if err != nil {
			return fmt.Errorf("failed to check API status: %v", err)
		}
		if !enabled {
			disabledAPIs = append(disabledAPIs, api)
		}
	}

	if len(disabledAPIs) > 0 {
		fmt.Println("The following required APIs are not enabled:")
		for _, api := range disabledAPIs {
			fmt.Printf("- %s\n", api)
		}
		fmt.Println("\nTo enable these APIs, please visit the following URLs:")
		for _, api := range disabledAPIs {
			fmt.Printf(
				"https://console.cloud.google.com/apis/library/%s?project=%s\n",
				api,
				adminProjectID,
			)
		}
		fmt.Println("\nAfter enabling the APIs, please run the command again.")
		return fmt.Errorf("required APIs are not enabled")
	}

	fmt.Println("All required APIs are enabled.")
	return nil
}
