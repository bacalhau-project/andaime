package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetGCPCreateProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-project",
		Short: "Create a new GCP project with necessary resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, _ := cmd.Flags().GetString("project-id")
			organizationID := viper.GetString("gcp.organization_id")
			if organizationID == "" {
				return fmt.Errorf("organization_id is not set in the configuration")
			}
			m := display.GetGlobalModelFunc()
			if m == nil || m.Deployment == nil {
				return fmt.Errorf("global model or deployment is nil")
			}
			m.Deployment.SetProjectID(projectID)

			return createProject(cmd.Context())
		},
	}

	cmd.Flags().StringP("project-id", "i", "", "The ID of the project to create")
	_ = cmd.MarkFlagRequired("project-id")

	return cmd
}

func createProject(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	m.Deployment.GCP.BillingAccountID = gcpProvider.BillingAccountID
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// Use EnsureProject to create or reuse an existing project
	err = gcpProvider.EnsureProject(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure project: %w", err)
	}

	fmt.Printf("Project ensured successfully: %s\n", m.Deployment.GetProjectID())

	// Update status of all machines with the project being created
	for i := range m.Deployment.GetMachines() {
		m.Deployment.GetMachine(i).SetStatusMessage(
			fmt.Sprintf(
				"Associated with project: %s",
				m.Deployment.GetProjectID(),
			),
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

	fmt.Println("Enabling necessary APIs...")
	for _, api := range apisToEnable {
		if err := gcpProvider.EnableAPI(ctx, api); err != nil {
			return handleGCPError(fmt.Errorf("failed to enable API %s: %v", api, err))
		}
		fmt.Printf("Enabled API: %s\n", api)
	}

	// Create or ensure VPC network
	networkName := "andaime-network"
	fmt.Printf("Ensuring VPC network: %s\n", networkName)
	err = gcpProvider.EnsureVPCNetwork(ctx, networkName)
	if err != nil {
		return handleGCPError(fmt.Errorf("failed to ensure VPC network: %v", err))
	}

	fmt.Printf("VPC network ensured successfully: %s\n", networkName)

	// Create or ensure firewall rules
	fmt.Println("Ensuring firewall rules...")
	if err := gcpProvider.EnsureFirewallRules(ctx, networkName); err != nil {
		return handleGCPError(fmt.Errorf("failed to ensure firewall rules: %v", err))
	}

	fmt.Println("Project setup completed successfully.")
	return nil
}
