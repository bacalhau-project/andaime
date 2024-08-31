package gcp

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GCPClienter interface {
	EnsureProject(
		ctx context.Context,
		projectID string,
	) (string, error)
	DestroyProject(ctx context.Context, projectID string) error
	ListProjects(
		ctx context.Context,
		req *resourcemanagerpb.ListProjectsRequest,
	) ([]*resourcemanagerpb.Project, error)
	ListAllResourcesInProject(ctx context.Context, projectID string) ([]interface{}, error)
	StartResourcePolling(ctx context.Context) error
	DeployResources(ctx context.Context) error
	ProvisionPackagesOnMachines(ctx context.Context) error
	ProvisionBacalhau(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
}

type LiveGCPClient struct {
	projectClient *resourcemanager.ProjectsClient
}

var NewGCPClientFunc = NewGCPClient

func NewGCPClient(ctx context.Context, organizationID string) (GCPClienter, error) {
	l := logger.Get()

	if err := checkCredentials(organizationID); err != nil {
		l.Errorf("Credential check failed: %v", err)
		return nil, err
	}

	log.Println("DEBUG: Creating projects client")
	projectClient, err := resourcemanager.NewProjectsClient(ctx, option.WithCredentialsFile(""))
	if err != nil {
		l.Errorf("Failed to create projects client: %v", err)
		return nil, fmt.Errorf("failed to create projects client: %v", err)
	}

	log.Println("DEBUG: GCP client initialized successfully")
	return &LiveGCPClient{
		projectClient: projectClient,
	}, nil
}

func (c *LiveGCPClient) EnsureProject(
	ctx context.Context,
	projectID string,
) (string, error) {
	l := logger.Get()

	timestamp := time.Now().Format("0601021504") // yymmddhhmm
	uniqueProjectID := fmt.Sprintf("%s-%s", projectID, timestamp)

	l.Debugf("Ensuring project: %s", uniqueProjectID)

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: &resourcemanagerpb.Project{
			ProjectId: uniqueProjectID,
			Labels: map[string]string{
				"created-by-andaime": "true",
			},
		},
	}

	l.Debugf("Creating project: %s", uniqueProjectID)
	createCallResponse, err := c.projectClient.CreateProject(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			l.Debugf("Project %s already exists, checking permissions", uniqueProjectID)

			// Check if we have permissions on the existing project
			getReq := &resourcemanagerpb.GetProjectRequest{
				Name: req.Project.ProjectId,
			}
			_, err := c.projectClient.GetProject(ctx, getReq)
			if err != nil {
				l.Errorf("Project exists but user doesn't have permissions: %v", err)
				return "", fmt.Errorf("project exists but user doesn't have permissions: %v", err)
			}

			l.Debugf("User has permissions on existing project %s", req.Project.ProjectId)
			return req.Project.ProjectId, nil
		}
		l.Errorf("Failed to create project: %v", err)
		return "", fmt.Errorf("create project: %v", err)
	}

	l.Debugf("Waiting for project creation to complete")
	_, err = createCallResponse.Wait(ctx)
	if err != nil {
		l.Errorf("Failed to wait for project creation: %v", err)
		return "", fmt.Errorf("wait for project creation: %v", err)
	}

	l.Debugf("Created project: %s", req.Project.ProjectId)
	return req.Project.ProjectId, nil
}

func (c *LiveGCPClient) DestroyProject(ctx context.Context, projectID string) error {
	log.Printf("DEBUG: Attempting to destroy project: %s", projectID)

	deleteOp, err := c.projectClient.DeleteProject(ctx, &resourcemanagerpb.DeleteProjectRequest{
		Name: fmt.Sprintf("projects/%s", projectID),
	})
	if err != nil {
		log.Printf("DEBUG: Failed to initiate project deletion: %v", err)
		return fmt.Errorf("failed to initiate project deletion: %v", err)
	}

	log.Println("DEBUG: Waiting for project deletion to complete")
	if _, err := deleteOp.Wait(ctx); err != nil {
		log.Printf("DEBUG: Failed to delete project: %v", err)
		return fmt.Errorf("failed to delete project: %v", err)
	}

	log.Printf("DEBUG: Project %s deleted successfully", projectID)
	return nil
}

func checkCredentials(parent string) error {
	if parent == "" {
		return fmt.Errorf("parent is required. Please specify a parent organization or folder")
	}

	l := logger.Get()
	l.Debug("Checking credentials")
	// Attempt to list projects to verify credentials
	ctx := context.Background()
	client, err := resourcemanager.NewProjectsClient(ctx, option.WithCredentialsFile(""))
	if err != nil {
		l.Debugf("Failed to create projects client: %v", err)
		return fmt.Errorf("failed to create projects client: %v", err)
	}
	defer client.Close()

	l.Debug("Attempting to list projects")
	it := client.ListProjects(ctx, &resourcemanagerpb.ListProjectsRequest{
		Parent: fmt.Sprintf("organizations/%s", parent),
	})
	// We only need to check if we can list at least one project
	_, err = it.Next()
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

func (c *LiveGCPClient) ListProjects(
	ctx context.Context,
	req *resourcemanagerpb.ListProjectsRequest,
) ([]*resourcemanagerpb.Project, error) {
	resp := c.projectClient.ListProjects(ctx, req)
	if resp == nil {
		return nil, fmt.Errorf("failed to list projects")
	}

	allProjects := []*resourcemanagerpb.Project{}
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

func (c *LiveGCPClient) StartResourcePolling(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	resourceTicker := time.NewTicker(ResourcePollingInterval)
	defer resourceTicker.Stop()

	for {
		select {
		case <-resourceTicker.C:
			if m.Quitting {
				l.Debug("Quitting detected, stopping resource polling")
				return nil
			}

			// Query all resources in the project
			resources, err := c.ListAllResourcesInProject(ctx, m.Deployment.ProjectID)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
				return err
			}

			l.Debugf("Poll: Found %d resources", len(resources))

			allResourcesProvisioned := true
			for _, resource := range resources {
				resourceMap := resource.(map[string]interface{})
				resourceType := resourceMap["type"].(string)
				resourceName := resourceMap["name"].(string)
				state := resourceMap["state"].(string)

				l.Debugf("Resource: %s (Type: %s) - State: %s", resourceName, resourceType, state)

				// Update the resource state in the deployment model
				if err := m.UpdateResourceState(resourceName, resourceType, state); err != nil {
					l.Errorf("Failed to update resource state: %v", err)
				}

				if state != "READY" {
					allResourcesProvisioned = false
				}
			}

			if allResourcesProvisioned && c.allMachinesComplete(m) {
				l.Debug(
					"All resources provisioned and machines completed, stopping resource polling",
				)
				return nil
			}

		case <-ctx.Done():
			l.Debug("Context cancelled, exiting resource polling")
			return ctx.Err()
		}
	}
}

func (c *LiveGCPClient) allMachinesComplete(m *display.DisplayModel) bool {
	for _, machine := range m.Deployment.Machines {
		if !machine.Complete() {
			return false
		}
	}
	return true
}

func (c *LiveGCPClient) DeployResources(ctx context.Context) error {
	// TODO: Implement resource deployment logic
	return fmt.Errorf("DeployResources not implemented")
}

func (c *LiveGCPClient) ProvisionPackagesOnMachines(ctx context.Context) error {
	// TODO: Implement package provisioning logic
	return fmt.Errorf("ProvisionPackagesOnMachines not implemented")
}

func (c *LiveGCPClient) ProvisionBacalhau(ctx context.Context) error {
	// TODO: Implement Bacalhau provisioning logic
	return fmt.Errorf("ProvisionBacalhau not implemented")
}

func (c *LiveGCPClient) FinalizeDeployment(ctx context.Context) error {
	// TODO: Implement deployment finalization logic
	return fmt.Errorf("FinalizeDeployment not implemented")
}

func (c *LiveGCPClient) ListAllResourcesInProject(
	ctx context.Context,
	projectID string,
) ([]interface{}, error) {
	l := logger.Get()
	l.Debugf("Listing all resources in project: %s", projectID)

	client, err := asset.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset client: %v", err)
	}
	defer client.Close()

	req := &assetpb.SearchAllResourcesRequest{
		Scope: fmt.Sprintf("projects/%s", projectID),
	}

	var resources []interface{}
	it := client.SearchAllResources(ctx, req)
	for {
		resource, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %v", err)
		}

		resourceMap := map[string]interface{}{
			"name":  resource.GetName(),
			"type":  resource.GetAssetType(),
			"state": resource.GetState(),
		}
		resources = append(resources, resourceMap)
	}

	return resources, nil
}
