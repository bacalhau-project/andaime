package gcp

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
)

func createDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-deployment",
		Short: "Create a new deployment in GCP",
		Long:  `Create a new deployment in Google Cloud Platform (GCP).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateDeployment()
		},
	}

	return cmd
}

func runCreateDeployment() error {
	fmt.Println("Creating deployment in GCP...")

	ctx := context.Background()
	service, err := cloudresourcemanager.NewService(
		ctx,
		option.WithCredentialsFile("path/to/your/credentials.json"),
	)
	if err != nil {
		log.Fatalf("Failed to create Resource Manager service: %v", err)
	}

	projectsService := cloudresourcemanager.NewProjectsService(service)
	project := &cloudresourcemanager.Project{
		ProjectId: "your-new-project-id",
		Name:      "Your New Project Name",
	}

	operation, err := projectsService.Create(project).Do()
	if err != nil {
		log.Fatalf("Failed to create project: %v", err)
	}

	fmt.Printf("Project creation operation: %s\n", operation.Name)
	// You'll likely need to wait for the operation to complete
	// before the project is fully usable.
	return nil
}
