package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *LiveGCPClient) CheckAuthentication(
	ctx context.Context,
	projectID string,
) error {
	rmService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Resource Manager client: %v", err)
	}

	_, err = rmService.Projects.List().PageSize(1).Do()
	if err != nil {
		return fmt.Errorf("authentication failed: %v", err)
	}

	return nil
}

func (c *LiveGCPClient) CheckPermissions(
	ctx context.Context,
	projectID string,
) error {
	l := logger.Get()
	l.Debug("Checking GCP permissions")

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	if projectID == "" {
		return fmt.Errorf("project ID is empty")
	}

	l.Debugf("Checking permissions for project: %s", projectID)

	requiredPermissions := []string{
		"cloudasset.assets.searchAllResources",
		"compute.instances.list",
		"resourcemanager.projects.get",
		"resourcemanager.projects.create",
		"resourcemanager.projects.delete",
		"billing.accounts.list",
		"serviceusage.services.enable",
	}

	request := &cloudresourcemanager.TestIamPermissionsRequest{
		Permissions: requiredPermissions,
	}
	response, err := c.resourceManagerService.Projects.TestIamPermissions("projects/"+projectID, request).
		Context(ctx).
		Do()

	if err != nil {
		l.Errorf("Failed to test IAM permissions: %v", err)
		if gerr, ok := err.(*googleapi.Error); ok {
			l.Errorf("Google API Error: %v", gerr.Message)
			l.Errorf("Error Details: %+v", gerr.Details)
		}
		return fmt.Errorf("failed to test IAM permissions: %w", err)
	}

	missingPermissions := utils.Difference(requiredPermissions, response.Permissions)
	if len(missingPermissions) > 0 {
		l.Errorf("Missing required permissions: %v", missingPermissions)
		return fmt.Errorf("missing required permissions: %v", missingPermissions)
	}

	l.Debug("All required permissions are granted")
	return nil
}

func (c *LiveGCPClient) CheckSpecificPermission(
	ctx context.Context,
	permission string,
	projectID string,
) error {
	l := logger.Get()
	l.Debugf("Checking specific permission: %s", permission)

	if projectID == "" {
		return fmt.Errorf("project ID is empty")
	}

	request := &cloudresourcemanager.TestIamPermissionsRequest{
		Permissions: []string{permission},
	}
	response, err := c.resourceManagerService.Projects.TestIamPermissions("projects/"+projectID, request).
		Context(ctx).
		Do()

	if err != nil {
		l.Errorf("Failed to test specific IAM permission: %v", err)
		return fmt.Errorf("failed to test specific IAM permission: %w", err)
	}

	if len(response.Permissions) == 0 {
		l.Errorf("Missing required permission: %s", permission)
		return fmt.Errorf("missing required permission: %s", permission)
	}

	l.Debugf("Permission %s is granted", permission)
	return nil
}

func (c *LiveGCPClient) CheckProjectAccess(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Debugf("Checking access to project: %s", projectID)

	service, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resource manager client: %v", err)
	}

	_, err = service.Projects.Get(projectID).Do()
	if err != nil {
		l.Errorf("Failed to access project %s: %v", projectID, err)
		return fmt.Errorf("failed to access project %s: %v", projectID, err)
	}

	l.Infof("Successfully accessed project: %s", projectID)
	return nil
}

func (c *LiveGCPClient) ProjectExists(ctx context.Context, projectID string) (bool, error) {
	l := logger.Get()
	l.Debugf("Checking if project %s exists", projectID)

	req := &resourcemanagerpb.GetProjectRequest{
		Name: fmt.Sprintf("projects/%s", projectID),
	}

	_, err := c.projectClient.GetProject(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		if status.Code(err) == codes.PermissionDenied {
			l.Errorf("Permission denied when checking project existence. Error: %v", err)
			return false, fmt.Errorf("permission denied when checking project existence: %w", err)
		}
		return false, fmt.Errorf("failed to check if project exists: %w", err)
	}

	return true, nil
}

func (c *LiveGCPClient) checkCredentials(ctx context.Context) error {
	if c.parentString == "" {
		return fmt.Errorf("parent is required. Please specify a parent organization or folder")
	}

	l := logger.Get()
	l.Debug("Checking credentials")

	l.Debug("Attempting to list projects")
	it := c.projectClient.ListProjects(ctx, &resourcemanagerpb.ListProjectsRequest{
		Parent: c.parentString,
	})
	_, err := it.Next()
	if err != nil && err != iterator.Done {
		l.Debugf("Failed to list projects: %v", err)
		return fmt.Errorf(
			"failed to list projects, which suggests an issue with credentials or permissions: %v\n%s",
			err,
			credentialsNotSetError,
		)
	}

	l.Debug("Successfully verified credentials")
	return nil
}

//nolint:gosec
const credentialsNotSetError = `
GOOGLE_APPLICATION_CREDENTIALS is not set. Please set up your credentials using the following steps:
1. Install the gcloud CLI if you haven't already: https://cloud.google.com/sdk/docs/install
2. Run the following commands in your terminal:
   gcloud auth login
   gcloud auth application-default login
3. The above command will set up your user credentials.
After completing these steps, run your application again.`
