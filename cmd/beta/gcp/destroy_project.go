package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/option"
)

func GetDestroyProjectCmd() *cobra.Command {
	return destroyProjectCmd()
}

func destroyProjectCmd() *cobra.Command {
	var deleteAll bool

	cmd := &cobra.Command{
		Use:   "destroy-project [PROJECT_ID]",
		Short: "Destroy GCP project(s)",
		Long:  `Destroy an existing Google Cloud Platform (GCP) project or all projects labeled with 'andaime'.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

	return cmd
}

func runDestroyProject(projectID string) error {
	ctx := context.Background()

	c, err := cloudresourcemanager.NewService(ctx, option.WithCredentialsFile(""))
	if err != nil {
		return fmt.Errorf("NewService: %v", err)
	}

	projectName := fmt.Sprintf("projects/%s", projectID)

	_, err = c.Projects.Delete(projectName).Do()
	if err != nil {
		return fmt.Errorf("failed to delete project: %v", err)
	}

	fmt.Printf("Project %s has been scheduled for deletion\n", projectID)
	fmt.Println("Note: The actual deletion may take some time to complete.")
	return nil
}

func runDestroyAllProjects() error {
	ctx := context.Background()

	p, err := gcp.NewGCPProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to create GCP provider: %v", err)
	}

	resp, err := p.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %v", err)
	}

	for _, project := range resp {
		if strings.Contains(project.Labels["created-by-andaime"], "true") {
			fmt.Printf("Deleting project: %s\n", project.ProjectId)
			err := p.DestroyProject(ctx, project.ProjectId)
			if err != nil {
				fmt.Printf("Failed to delete project %s: %v\n", project.ProjectId, err)
			} else {
				fmt.Printf("Project %s has been scheduled for deletion\n", project.ProjectId)
			}
		}
	}

	fmt.Println("All matching projects have been scheduled for deletion.")
	fmt.Println("Note: The actual deletion may take some time to complete.")
	return nil
}
