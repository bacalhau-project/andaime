package gcp

import (
	"context"
	"fmt"

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
			return createProject(projectID)
		},
	}

	cmd.Flags().StringP("project-id", "i", "", "The ID of the project to create")
	_ = cmd.MarkFlagRequired("project-id")

	return cmd
}

func createProject(projectID string) error {
	ctx := context.Background()
	p, err := gcp.NewGCPProviderFunc()
	if err != nil {
		return handleGCPError(err)
	}

	// Use client directly instead of through a provider
	createdProjectID, err := p.EnsureProject(ctx, projectID)
	if err != nil {
		return handleGCPError(err)
	}

	fmt.Printf("Project created successfully: %s\n", createdProjectID)

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
		if err := p.EnableAPI(ctx, createdProjectID, api); err != nil {
			return handleGCPError(fmt.Errorf("failed to enable API %s: %v", api, err))
		}
		fmt.Printf("Enabled API: %s\n", api)
	}

	// Create VPC network
	networkName := "andaime-network"
	fmt.Printf("Creating VPC network: %s\n", networkName)
	if err := p.CreateVPCNetwork(ctx, createdProjectID, networkName); err != nil {
		return handleGCPError(fmt.Errorf("failed to create VPC network: %v", err))
	}

	// Create subnet
	subnetName := "andaime-subnet"
	subnetCIDR := "10.0.0.0/24"
	fmt.Printf("Creating subnet: %s with CIDR %s\n", subnetName, subnetCIDR)
	if err := p.CreateSubnet(ctx, createdProjectID, networkName, subnetName, subnetCIDR); err != nil {
		return handleGCPError(fmt.Errorf("failed to create subnet: %v", err))
	}

	// Create firewall rules
	fmt.Println("Creating firewall rules...")
	if err := p.CreateFirewallRules(ctx, createdProjectID, networkName); err != nil {
		return handleGCPError(fmt.Errorf("failed to create firewall rules: %v", err))
	}

	// Create Cloud Storage bucket
	bucketName := fmt.Sprintf("%s-storage", createdProjectID)
	fmt.Printf("Creating Cloud Storage bucket: %s\n", bucketName)
	if err := p.CreateStorageBucket(ctx, createdProjectID, bucketName); err != nil {
		return handleGCPError(fmt.Errorf("failed to create storage bucket: %v", err))
	}

	fmt.Println("Project setup completed successfully.")
	return nil
}
