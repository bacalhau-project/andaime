package gcp

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func (c *LiveGCPClient) EnsureProject(
	ctx context.Context,
	organizationID string,
	projectID string,
	billingAccountID string,
) (string, error) {
	l := logger.Get()

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: &resourcemanagerpb.Project{
			ProjectId:   projectID,
			DisplayName: projectID,
			Parent:      c.GetParentString(),
			Labels:      map[string]string{"deployed-by": "andaime"},
		},
	}

	l.Infof("Creating or ensuring project: %s", projectID)
	createCallResponse, err := c.projectClient.CreateProject(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return c.checkExistingProjectPermissions(ctx, projectID)
		}
		return "", fmt.Errorf("create project: %v", err)
	}

	project, err := createCallResponse.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("wait for project creation: %v", err)
	}

	l.Infof("Created project: %s (Display Name: %s)", project.ProjectId, project.DisplayName)

	if err := c.setupProject(ctx, organizationID, projectID, billingAccountID); err != nil {
		return "", err
	}

	return project.ProjectId, nil
}

func (c *LiveGCPClient) setupProject(
	ctx context.Context,
	organizationID string,
	projectID string,
	billingAccountID string,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	if err := c.SetBillingAccount(ctx, projectID, billingAccountID); err != nil {
		return fmt.Errorf("failed to set billing account: %v", err)
	}

	if err := c.enableRequiredAPIs(ctx, projectID); err != nil {
		return fmt.Errorf("failed to enable required APIs: %v", err)
	}

	if err := c.setupServiceAccount(ctx, projectID); err != nil {
		return fmt.Errorf("failed to set up service account: %v", err)
	}

	if err := c.testProjectPermissions(ctx, projectID); err != nil {
		return fmt.Errorf("failed to verify project permissions: %v", err)
	}

	return nil
}

func (c *LiveGCPClient) checkExistingProjectPermissions(
	ctx context.Context,
	projectID string,
) (string, error) {
	l := logger.Get()
	getReq := &resourcemanagerpb.GetProjectRequest{
		Name: fmt.Sprintf("projects/%s", projectID),
	}
	existingProject, err := c.projectClient.GetProject(ctx, getReq)
	if err != nil {
		l.Errorf("Project exists but user doesn't have permissions: %v", err)
		return "", fmt.Errorf("project exists but user doesn't have permissions: %v", err)
	}

	l.Debugf("User has permissions on existing project %s", projectID)

	return existingProject.ProjectId, nil
}

func (c *LiveGCPClient) testProjectPermissions(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Testing permissions and resource creation for project %s", projectID)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = maxBackOffTime

	return backoff.Retry(func() error {
		if err := c.TestComputeEngineAPI(ctx, projectID, b.GetElapsedTime(), b.MaxElapsedTime); err != nil {
			return err
		}
		if err := c.TestCloudStorageAPI(ctx, projectID); err != nil {
			return err
		}
		if err := c.TestIAMPermissions(ctx, projectID); err != nil {
			return err
		}
		if err := c.TestServiceUsageAPI(ctx, projectID); err != nil {
			return err
		}
		return nil
	}, b)
}

func (c *LiveGCPClient) DestroyProject(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Attempting to destroy project: %s", projectID)

	deleteOp, err := c.projectClient.DeleteProject(ctx, &resourcemanagerpb.DeleteProjectRequest{
		Name: fmt.Sprintf("projects/%s", projectID),
	})
	if err != nil {
		l.Errorf("Failed to initiate project deletion: %v", err)
		return fmt.Errorf("failed to initiate project deletion: %v", err)
	}

	l.Infof("Waiting for project deletion to complete")
	if _, err := deleteOp.Wait(ctx); err != nil {
		l.Errorf("Failed to delete project: %v", err)
		return fmt.Errorf("failed to delete project: %v", err)
	}

	l.Infof("Project %s deleted successfully", projectID)
	return nil
}

func (c *LiveGCPClient) ListProjects(
	ctx context.Context,
	req *resourcemanagerpb.ListProjectsRequest,
) ([]*resourcemanagerpb.Project, error) {
	l := logger.Get()
	l.Debug("Attempting to list projects")
	req.Parent = c.parentString
	resp := c.projectClient.ListProjects(ctx, req)
	if resp == nil {
		return nil, fmt.Errorf("failed to list projects")
	}

	var allProjects []*resourcemanagerpb.Project
	for {
		project, err := resp.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %v", err)
		}
		allProjects = append(allProjects, project)
	}
	return allProjects, nil
}
