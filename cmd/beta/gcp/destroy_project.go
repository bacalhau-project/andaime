package gcp

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetGCPDestroyProjectCmd() *cobra.Command {
	var deleteAll bool
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "destroy-project [PROJECT_ID]",
		Short: "Destroy GCP project(s) or list all projects",
		Long: `Destroy an existing Google Cloud Platform (GCP) project, 
all projects labeled with 'andaime', or list all available projects.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// This will prevent the default error handling
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			var err error
			if listOnly {
				err = listAllProjects()
			} else if deleteAll {
				err = runDestroyAllProjects()
			} else if len(args) == 0 {
				err = fmt.Errorf("PROJECT_ID is required when --all flag is not set")
			} else {
				err = runDestroyProject(cmd, args[0])
			}

			if err != nil {
				fmt.Println(err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&deleteAll, "all", false, "Delete all projects labeled with 'andaime'")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List all available projects without deleting")

	return cmd
}

func listAllProjects() error {
	ctx := context.Background()
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		viper.GetString("gcp.project_id"),
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}
	projects, err := gcpProvider.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %v", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects available.")
		return nil
	}

	fmt.Println("Available projects:")
	andaimeProjects := []*resourcemanagerpb.Project{}
	for _, project := range projects {
		fmt.Printf("- %s (ID: %s)\n", project.DisplayName, project.ProjectId)
		if project.Labels["deployed-by"] == "andaime" {
			andaimeProjects = append(andaimeProjects, project)
		}
	}

	fmt.Println("\nProjects that would be deleted (labeled with 'deployed-by: andaime'):")
	if len(andaimeProjects) == 0 {
		fmt.Println("No projects found with 'deployed-by: andaime' label.")
	} else {
		for _, project := range andaimeProjects {
			fmt.Printf("- %s (ID: %s)\n", project.DisplayName, project.ProjectId)
		}
	}

	return nil
}

func runDestroyAllProjects() error {
	ctx := context.Background()
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		viper.GetString("gcp.project_id"),
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	projects, err := gcpProvider.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %v", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects available.")
		return nil
	}

	fmt.Println("Available projects:")
	andaimeProjects := []*resourcemanagerpb.Project{}
	for _, project := range projects {
		fmt.Printf("- %s (ID: %s)\n", project.DisplayName, project.ProjectId)
		if project.Labels["deployed-by"] == "andaime" {
			andaimeProjects = append(andaimeProjects, project)
		}
	}

	fmt.Println("\nProjects to be deleted (labeled with 'deployed-by: andaime'):")
	if len(andaimeProjects) == 0 {
		fmt.Println("No projects found with 'deployed-by: andaime' label.")
		return nil
	}

	for _, project := range andaimeProjects {
		fmt.Printf("- %s (ID: %s)\n", project.DisplayName, project.ProjectId)
	}

	fmt.Print("\nAre you sure you want to delete these projects? (y/N): ")
	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil {
		return fmt.Errorf("failed to read confirmation: %v", err)
	}
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	for _, project := range andaimeProjects {
		fmt.Printf("Deleting project: %s\n", project.ProjectId)
		err := gcpProvider.DestroyProject(ctx, project.ProjectId)
		if err != nil {
			fmt.Printf("Failed to delete project %s: %v\n", project.ProjectId, err)
		} else {
			fmt.Printf("Project %s has been scheduled for deletion\n", project.ProjectId)
		}
	}

	fmt.Println("All matching projects have been scheduled for deletion.")
	fmt.Println("Note: The actual deletion may take some time to complete.")
	return nil
}

func runDestroyProject(cmd *cobra.Command, projectID string) error {
	ctx := context.Background()
	gcpProvider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		projectID,
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	fmt.Printf("Are you sure you want to delete project %s? (y/N): ", projectID)
	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil {
		return fmt.Errorf("failed to read confirmation: %v", err)
	}
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	err = gcpProvider.DestroyProject(ctx, projectID)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: Failed to delete project %s\n", projectID)
		fmt.Fprintf(cmd.ErrOrStderr(), "Reason: Authentication issue with Google Cloud\n")
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"Please ensure you're logged in and have the necessary permissions.\n",
		)
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"For more information, visit: https://support.google.com/a/answer/9368756\n",
		)
		return err
	}

	fmt.Printf("Project %s has been scheduled for deletion\n", projectID)
	fmt.Println("Note: The actual deletion may take some time to complete.")
	return nil
}
