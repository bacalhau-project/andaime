package gcp

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
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
	cmd.Flags().
		StringVarP(&projectName, "project-name", "n", "", "The name of the project to create")
	err := cmd.MarkFlagRequired("project-id")
	if err != nil {
		fmt.Println(err)
		return nil
	}
	err = cmd.MarkFlagRequired("project-name")
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return cmd
}

func runCreateProject(projectID, projectName string) error {
	ctx := context.Background()
	c, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return fmt.Errorf("NewService: %v", err)
	}

	req := &cloudresourcemanager.Project{
		ProjectId: projectID,
		Name:      projectName,
	}

	createCallResponse := c.Projects.Create(req)
	opts := []googleapi.CallOption{}

	resp, err := createCallResponse.Do(opts...)
	if err != nil {
		return fmt.Errorf("do: %v", err)
	}

	fmt.Printf("Created project: %s\n", resp.Name)
	return nil
}
