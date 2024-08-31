package gcp

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
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
	ListAllAssetsInProject(
		ctx context.Context,
		projectID string,
	) ([]*assetpb.Asset, error)
	StartResourcePolling(ctx context.Context) error
	DeployResources(ctx context.Context) error
	ProvisionPackagesOnMachines(ctx context.Context) error
	ProvisionBacalhau(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	CheckAuthentication(ctx context.Context) error
	EnableAPI(ctx context.Context, projectID, apiName string) error
	CreateVPCNetwork(ctx context.Context, projectID, networkName string) error
	CreateSubnet(ctx context.Context, projectID, networkName, subnetName, cidr string) error
	CreateFirewallRules(ctx context.Context, projectID, networkName string) error
	CreateStorageBucket(ctx context.Context, projectID, bucketName string) error
	CreateVM(ctx context.Context, projectID string, vmConfig map[string]string) (string, error)
	waitForOperation(
		ctx context.Context,
		service *compute.Service,
		project, zone, operation string,
	) error
}

type LiveGCPClient struct {
	parentString  string
	projectClient *resourcemanager.ProjectsClient
	assetClient   *asset.Client
}

var NewGCPClientFunc = NewGCPClient

func NewGCPClient(ctx context.Context, organizationID string) (GCPClienter, error) {
	l := logger.Get()

	log.Println("DEBUG: Creating projects client")
	projectClient, err := resourcemanager.NewProjectsClient(ctx, option.WithCredentialsFile(""))
	if err != nil {
		l.Errorf("Failed to create projects client: %v", err)
		return nil, fmt.Errorf("failed to create projects client: %v", err)
	}

	if err := checkCredentials(ctx, projectClient, organizationID); err != nil {
		l.Errorf("Credential check failed: %v", err)
		return nil, err
	}

	parentString := fmt.Sprintf("organizations/%s", organizationID)

	assetClient, err := asset.NewClient(ctx, option.WithCredentialsFile(""))
	if err != nil {
		l.Errorf("Failed to create asset client: %v", err)
		return nil, fmt.Errorf("failed to create asset client: %v", err)
	}

	log.Println("DEBUG: GCP client initialized successfully")
	return &LiveGCPClient{
		parentString:  parentString,
		projectClient: projectClient,
		assetClient:   assetClient,
	}, nil
}

func checkCredentials(
	ctx context.Context,
	client *resourcemanager.ProjectsClient,
	parent string,
) error {
	if parent == "" {
		return fmt.Errorf("parent is required. Please specify a parent organization or folder")
	}

	l := logger.Get()
	l.Debug("Checking credentials")

	l.Debug("Attempting to list projects")
	it := client.ListProjects(ctx, &resourcemanagerpb.ListProjectsRequest{
		Parent: fmt.Sprintf("organizations/%s", parent),
	})
	// We only need to check if we can list at least one project
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
			ProjectId:   uniqueProjectID,
			DisplayName: uniqueProjectID, // Set the display name to be the same as the project ID
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
				Name: fmt.Sprintf("projects/%s", uniqueProjectID),
			}
			existingProject, err := c.projectClient.GetProject(ctx, getReq)
			if err != nil {
				l.Errorf("Project exists but user doesn't have permissions: %v", err)
				return "", fmt.Errorf("project exists but user doesn't have permissions: %v", err)
			}

			l.Debugf("User has permissions on existing project %s", uniqueProjectID)
			return existingProject.ProjectId, nil
		}
		l.Errorf("Failed to create project: %v", err)
		return "", fmt.Errorf("create project: %v", err)
	}

	l.Debugf("Waiting for project creation to complete")
	project, err := createCallResponse.Wait(ctx)
	if err != nil {
		l.Errorf("Failed to wait for project creation: %v", err)
		return "", fmt.Errorf("wait for project creation: %v", err)
	}

	l.Debugf("Created project: %s (Display Name: %s)", project.ProjectId, project.DisplayName)
	return project.ProjectId, nil
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
			resources, err := c.ListAllAssetsInProject(ctx, m.Deployment.ProjectID)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
				return err
			}

			l.Debugf("Poll: Found %d resources", len(resources))

			allResourcesProvisioned := true
			for _, resource := range resources {
				resourceType := resource.GetAssetType()
				resourceName := resource.GetName()
				state := resource.GetServicePerimeter().Status

				l.Debugf("Resource: %s (Type: %s) - State: %s", resourceName, resourceType, state)

				// Update the resource state in the deployment model
				if err := c.UpdateResourceState(resourceName, resourceType, "NOT IMPLEMENTED"); err != nil {
					l.Errorf("Failed to update resource state: %v", err)
				}

				// if state != "READY" {
				// 	allResourcesProvisioned = false
				// }
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

func (c *LiveGCPClient) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	resources := []*assetpb.Asset{}
	l := logger.Get()
	l.Debugf("Listing all resources in project: %s", projectID)

	req := &assetpb.SearchAllResourcesRequest{
		Scope: fmt.Sprintf("projects/%s", projectID),
	}

	it := c.assetClient.SearchAllResources(ctx, req)
	for {
		resource, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %v", err)
		}

		resourceAsset := &assetpb.Asset{
			Name:       resource.GetName(),
			UpdateTime: resource.GetUpdateTime(),
			AssetType:  resource.GetAssetType(),
		}
		resources = append(resources, resourceAsset)
	}

	return resources, nil
}

// UpdateResourceState updates the state of a resource in the deployment
func (c *LiveGCPClient) UpdateResourceState(resourceName, resourceType, state string) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// // Find the machine that owns this resource
	// for _, machine := range m.Deployment.Machines {
	// 	if strings.HasPrefix(resourceName, machine.Name) {
	// 		// Update the resource state for this machine
	// 		if machine.EnsureMachineServices() == nil {
	// 			machine.Resources = make(map[string]models.Resource)
	// 		}
	// 		machine.Resources[resourceType] = models.Resource{
	// 			ResourceState: models.AzureResourceState(state),
	// 			ResourceValue: resourceName,
	// 		}
	// 		return nil
	// 	}
	// }

	return fmt.Errorf("resource %s not found in any machine", resourceName)
}

func (c *LiveGCPClient) CheckAuthentication(ctx context.Context) error {
	// Create a Resource Manager client
	rmService, err := cloudresourcemanager.NewService(
		ctx,
		option.WithScopes(cloudresourcemanager.CloudPlatformScope),
	)
	if err != nil {
		return fmt.Errorf("failed to create Resource Manager client: %v", err)
	}

	// Try to list projects (limited to 1) to check if we have the necessary permissions
	_, err = rmService.Projects.List().PageSize(1).Do()
	if err != nil {
		return fmt.Errorf("authentication failed: %v", err)
	}

	return nil
}

func (c *LiveGCPClient) EnableAPI(ctx context.Context, projectID, apiName string) error {
	return fmt.Errorf("EnableAPI not implemented")
}

func (c *LiveGCPClient) CreateVPCNetwork(ctx context.Context, projectID, networkName string) error {
	return fmt.Errorf("CreateVPCNetwork not implemented")
}

func (c *LiveGCPClient) CreateSubnet(
	ctx context.Context,
	projectID, networkName, subnetName, cidr string,
) error {
	return fmt.Errorf("CreateSubnet not implemented")
}

func (c *LiveGCPClient) CreateFirewallRules(
	ctx context.Context,
	projectID, networkName string,
) error {
	return fmt.Errorf("CreateFirewallRules not implemented")
}

func (c *LiveGCPClient) CreateStorageBucket(
	ctx context.Context,
	projectID, bucketName string,
) error {
	return fmt.Errorf("CreateStorageBucket not implemented")
}

func (c *LiveGCPClient) CreateVM(
	ctx context.Context,
	projectID string,
	vmConfig map[string]string,
) (string, error) {
	l := logger.Get()
	l.Debugf("Creating VM in project: %s", projectID)

	// Ensure the necessary APIs are enabled
	if err := c.EnableAPI(ctx, projectID, "compute.googleapis.com"); err != nil {
		return "", fmt.Errorf("failed to enable Compute Engine API: %v", err)
	}

	// Create Compute Engine service client
	computeService, err := compute.NewService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create Compute Engine service client: %v", err)
	}

	// Generate a unique VM name
	uniqueID := fmt.Sprintf("%s-%s", projectID, time.Now().Format("20060102150405"))
	vmName := fmt.Sprintf("%s-vm", uniqueID)

	// Get default values from config
	defaultZone := viper.GetString("gcp.default_zone")
	defaultMachineType := viper.GetString("gcp.default_machine_type")
	defaultDiskSizeGB := viper.GetString("gcp.default_disk_size_gb")
	defaultSourceImage := viper.GetString("gcp.default_source_image")

	// Create or get the network
	networkName := fmt.Sprintf("%s-network", projectID)
	network, err := c.getOrCreateNetwork(ctx, computeService, projectID, networkName)
	if err != nil {
		return "", fmt.Errorf("failed to get or create network: %v", err)
	}

	// Convert defaultDiskSizeGB to int64
	defaultDiskSizeGBInt, err := strconv.ParseInt(defaultDiskSizeGB, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to convert default disk size to int64: %v", err)
	}

	instance := &compute.Instance{
		Name: vmName,
		MachineType: fmt.Sprintf(
			"zones/%s/machineTypes/%s",
			defaultZone,
			defaultMachineType,
		),
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskSizeGb:  defaultDiskSizeGBInt,
					SourceImage: defaultSourceImage,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: network.SelfLink,
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.ComputeScope,
				},
			},
		},
	}

	// Create the VM instance
	op, err := computeService.Instances.Insert(projectID, defaultZone, instance).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create VM instance: %v", err)
	}

	// Wait for the operation to complete
	err = c.waitForOperation(ctx, computeService, projectID, defaultZone, op.Name)
	if err != nil {
		return "", fmt.Errorf("failed to wait for VM creation: %v", err)
	}

	return vmName, nil
}

func (c *LiveGCPClient) getOrCreateNetwork(
	ctx context.Context,
	computeService *compute.Service,
	projectID, networkName string,
) (*compute.Network, error) {
	network, err := computeService.Networks.Get(projectID, networkName).Do()
	if err == nil {
		return network, nil
	}

	if !isNotFoundError(err) {
		return nil, fmt.Errorf("failed to get network: %v", err)
	}

	// Network doesn't exist, create it
	network = &compute.Network{
		Name:                  networkName,
		AutoCreateSubnetworks: true,
	}

	op, err := computeService.Networks.Insert(projectID, network).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %v", err)
	}

	err = c.waitForGlobalOperation(ctx, computeService, projectID, op.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for network creation: %v", err)
	}

	return computeService.Networks.Get(projectID, networkName).Do()
}

func (c *LiveGCPClient) waitForGlobalOperation(
	ctx context.Context,
	service *compute.Service,
	project, operation string,
) error {
	for {
		op, err := service.GlobalOperations.Get(project, operation).Do()
		if err != nil {
			return err
		}
		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("operation failed: %v", op.Error.Errors)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(ResourcePollingInterval)
		}
	}
}

func isNotFoundError(err error) bool {
	gerr, ok := err.(*googleapi.Error)
	return ok && gerr.Code == 404
}

func (c *LiveGCPClient) waitForOperation(
	ctx context.Context,
	service *compute.Service,
	project, zone, operation string,
) error {
	for {
		op, err := service.ZoneOperations.Get(project, zone, operation).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get operation status: %v", err)
		}

		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("operation failed: %v", op.Error.Errors)
			}
			return nil
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Wait before checking again
			time.Sleep(ResourcePollingInterval)
		}
	}
}
