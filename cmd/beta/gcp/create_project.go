package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/cobra"
)

func createProjectCmd() *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "create-project",
		Short: "Create a new GCP project",
		Long:  `Create a new Google Cloud Platform (GCP) project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateProject(projectID)
		},
	}

	cmd.Flags().StringVarP(&projectID, "project-id", "i", "", "The ID of the project to create")
	err := cmd.MarkFlagRequired("project-id")
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return cmd
}

func runCreateProject(projectID string) error {
	ctx := context.Background()

	p, err := gcp.NewGCPProviderFunc()
	if err != nil {
		return fmt.Errorf("NewGCPProviderFunc: %v", err)
	}

	project, err := p.EnsureProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("CreateProject: %v", err)
	}

	fmt.Printf("Project created: %v\n", project)
	return nil
}
