package gcp

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
)

func GetDestroyProjectCmd() *cobra.Command {
	var deleteAll bool
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "destroy-project [PROJECT_ID]",
		Short: "Destroy GCP project(s) or list all projects",
		Long: `Destroy an existing Google Cloud Platform (GCP) project, 
all projects labeled with 'andaime', or list all available projects.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listOnly {
				return listAllProjects()
			}
			if deleteAll {
				return runDestroyAllProjects()
			}
			if len(args) == 0 {
				return fmt.Errorf("PROJECT_ID is required when --all flag is not set")
			}
			return runDestroyProject(args[0])
		},
	}

	cmd.Flags().BoolVar(&deleteAll, "all", false, "Delete all projects labeled with 'andaime'")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List all available projects without deleting")

	return cmd
}

func listAllProjects() error {
	ctx := context.Background()
	p, err := gcp.NewGCPProviderFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCP provider: %v", err)
	}

	projects, err := p.ListProjects(ctx)
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
		if project.Labels["created-by-andaime"] == "true" {
			andaimeProjects = append(andaimeProjects, project)
		}
	}

	fmt.Println("\nProjects that would be deleted (labeled with 'created-by-andaime'):")
	if len(andaimeProjects) == 0 {
		fmt.Println("No projects found with 'created-by-andaime' label.")
	} else {
		for _, project := range andaimeProjects {
			fmt.Printf("- %s (ID: %s)\n", project.DisplayName, project.ProjectId)
		}
	}

	return nil
}

func runDestroyAllProjects() error {
	ctx := context.Background()
	p, err := gcp.NewGCPProviderFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCP provider: %v", err)
	}

	projects, err := p.ListProjects(ctx)
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
		if project.Labels["created-by-andaime"] == "true" {
			andaimeProjects = append(andaimeProjects, project)
		}
	}

	fmt.Println("\nProjects to be deleted (labeled with 'created-by-andaime'):")
	if len(andaimeProjects) == 0 {
		fmt.Println("No projects found with 'created-by-andaime' label.")
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
		err := p.DestroyProject(ctx, project.ProjectId)
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

func runDestroyProject(projectID string) error {
	ctx := context.Background()
	p, err := gcp.NewGCPProviderFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCP provider: %v", err)
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

	err = p.DestroyProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to delete project: %v", err)
	}

	fmt.Printf("Project %s has been scheduled for deletion\n", projectID)
	fmt.Println("Note: The actual deletion may take some time to complete.")
	return nil
}
