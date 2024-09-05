package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetCreateProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-project",
		Short: "Create a new GCP project with necessary resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, _ := cmd.Flags().GetString("project-id")
			organizationID := viper.GetString("gcp.organization_id")
			if organizationID == "" {
				return fmt.Errorf("organization_id is not set in the configuration")
			}
			return createProject(cmd.Context(), projectID)
		},
	}

	cmd.Flags().StringP("project-id", "i", "", "The ID of the project to create")
	_ = cmd.MarkFlagRequired("project-id")

	return cmd
}

func createProject(ctx context.Context, projectID string) error {
	m := display.GetGlobalModelFunc()
	p, err := gcp.NewGCPProviderFunc(ctx)
	if err != nil {
		return handleGCPError(err)
	}

	billingAccountID := viper.GetString("gcp.billing_account_id")
	if billingAccountID == "" {
		return fmt.Errorf("billing_account_id is not set in the configuration")
	}
	m.Deployment.GCP.BillingAccountID = billingAccountID

	// Use EnsureProject to create or reuse an existing project
	m.Deployment.ProjectID, err = p.EnsureProject(ctx, projectID)
	if err != nil {
		return handleGCPError(err)
	}

	fmt.Printf("Project ensured successfully: %s\n", m.Deployment.ProjectID)

	// Update status of all machines with the project being created
	for i := range m.Deployment.Machines {
		m.Deployment.Machines[i].StatusMessage = fmt.Sprintf(
			"Associated with project: %s",
			m.Deployment.ProjectID,
		)
	}

	// Enable necessary APIs
	apisToEnable := []string{
		"compute.googleapis.com",
		"storage-api.googleapis.com",
		"storage-component.googleapis.com",
		"networkmanagement.googleapis.com",
		"servicenetworking.googleapis.com",
		"cloudfunctions.googleapis.com",
		"cloudresourcemanager.googleapis.com",
	}

	// Set Billing Account
	if err := p.SetBillingAccount(ctx); err != nil {
		return handleGCPError(fmt.Errorf("failed to set billing account: %v", err))
	}

	fmt.Println("Billing account set successfully.")

	fmt.Println("Enabling necessary APIs...")
	for _, api := range apisToEnable {
		if err := p.EnableAPI(ctx, api); err != nil {
			return handleGCPError(fmt.Errorf("failed to enable API %s: %v", api, err))
		}
		fmt.Printf("Enabled API: %s\n", api)
	}

	// Create or ensure VPC network
	networkName := "andaime-network"
	fmt.Printf("Ensuring VPC network: %s\n", networkName)
	err = p.GetGCPClient().EnsureVPCNetwork(ctx, networkName)
	if err != nil {
		return handleGCPError(fmt.Errorf("failed to ensure VPC network: %v", err))
	}

	fmt.Printf("VPC network ensured successfully: %s\n", networkName)

	// Create or ensure firewall rules
	fmt.Println("Ensuring firewall rules...")
	if err := p.EnsureFirewallRules(ctx, networkName); err != nil {
		return handleGCPError(fmt.Errorf("failed to ensure firewall rules: %v", err))
	}

	fmt.Println("Project setup completed successfully.")
	return nil
}
