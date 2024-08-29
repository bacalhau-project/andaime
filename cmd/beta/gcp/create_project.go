package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/cloudresourcemanager/apiv3"
	"github.com/spf13/cobra"
	resourcemanagerpb "google.golang.org/genproto/googleapis/cloud/resourcemanager/v3"
)

func createProjectCmd() *cobra.Command {
	var projectID string
	var projectName string

	cmd := &cobra.Command{
		Use:   "create-project",
		Short: "Create a new GCP project",
		Long:  `Create a new Google Cloud Platform (GCP) project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateProject(projectID, projectName)
		},
	}

	cmd.Flags().StringVarP(&projectID, "project-id", "i", "", "The ID of the project to create")
	cmd.Flags().StringVarP(&projectName, "project-name", "n", "", "The name of the project to create")
	cmd.MarkFlagRequired("project-id")
	cmd.MarkFlagRequired("project-name")

	return cmd
}

func runCreateProject(projectID, projectName string) error {
	ctx := context.Background()
	c, err := cloudresourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return fmt.Errorf("NewProjectsClient: %v", err)
	}
	defer c.Close()

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: &resourcemanagerpb.Project{
			ProjectId: projectID,
			DisplayName: projectName,
		},
	}

	op, err := c.CreateProject(ctx, req)
	if err != nil {
		return fmt.Errorf("CreateProject: %v", err)
	}

	resp, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("Wait: %v", err)
	}

	fmt.Printf("Created project: %s\n", resp.Name)
	return nil
}
