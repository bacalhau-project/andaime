package gcp

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	billing "cloud.google.com/go/billing/apiv1"
	"cloud.google.com/go/billing/apiv1/billingpb"
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	"cloud.google.com/go/serviceusage/apiv1/serviceusagepb"
	"cloud.google.com/go/storage"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
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
	CreateVPCNetwork(ctx context.Context, networkName string) error
	CreateFirewallRules(ctx context.Context, networkName string) error
	CreateStorageBucket(ctx context.Context, bucketName string) error
	CreateVM(ctx context.Context, vmConfig map[string]string) (string, error)
	waitForOperation(
		ctx context.Context,
		project, zone, operation string,
	) error
	SetBillingAccount(ctx context.Context, projectID, billingAccountID string) error
	ListBillingAccounts(ctx context.Context) ([]string, error)
	CreateServiceAccount(
		ctx context.Context,
		projectID string,
		saName string,
	) (*iam.ServiceAccount, error)
	CreateServiceAccountKey(
		ctx context.Context,
		projectID, serviceAccountEmail string,
	) (*iam.ServiceAccountKey, error)
	waitForRegionalOperation(
		ctx context.Context,
		project, region, operation string,
	) error
	IsAPIEnabled(ctx context.Context, projectID, apiName string) (bool, error)
	GetVMExternalIP(ctx context.Context, projectID, zone, vmName string) (string, error)
	waitForGlobalOperation(
		ctx context.Context,
		project, operation string,
	) error
}

type LiveGCPClient struct {
	parentString           string
	projectClient          *resourcemanager.ProjectsClient
	assetClient            *asset.Client
	cloudBillingClient     *billing.CloudBillingClient
	iamService             *iam.Service
	serviceUsageClient     *serviceusage.Client
	computeClient          *compute.InstancesClient
	networksClient         *compute.NetworksClient
	firewallsClient        *compute.FirewallsClient
	zoneOperationsClient   *compute.ZoneOperationsClient
	globalOperationsClient *compute.GlobalOperationsClient
	regionOperationsClient *compute.RegionOperationsClient
}

var NewGCPClientFunc = NewGCPClient

type CloseableClient interface {
	Close() error
}

func NewGCPClient(ctx context.Context, organizationID string) (GCPClienter, func(), error) {
	l := logger.Get()

	// Centralized credential handling (adjust as needed)
	creds, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find default credentials: %v", err)
	}

	clientOpts := []option.ClientOption{
		option.WithCredentials(creds),
	}

	// List of clients to be cleaned up
	clientList := []CloseableClient{}

	// Create clients using the helper function
	projectClient, err := resourcemanager.NewProjectsClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, projectClient)

	assetClient, err := asset.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, assetClient)

	cloudBillingClient, err := billing.NewCloudBillingClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, cloudBillingClient)

	serviceUsageClient, err := serviceusage.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, serviceUsageClient)
	computeClient, err := compute.NewInstancesRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, computeClient)
	// Credential check and parent string setup (replace with your actual implementation)
	if err := checkCredentials(ctx, projectClient, organizationID); err != nil {
		l.Errorf("Credential check failed: %v", err)
		// Close clients on error
		for _, client := range clientList {
			client.Close()
		}
		return nil, nil, err
	}

	firewallsClient, err := compute.NewFirewallsRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, firewallsClient)

	networksClient, err := compute.NewNetworksRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, networksClient)

	zoneOperationsClient, err := compute.NewZoneOperationsRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, zoneOperationsClient)

	globalOperationsClient, err := compute.NewGlobalOperationsRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, globalOperationsClient)

	regionOperationsClient, err := compute.NewRegionOperationsRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, regionOperationsClient)

	parentString := fmt.Sprintf("organizations/%s", organizationID)

	// Cleanup function to close all clients
	cleanup := func() {
		l.Debug("Cleaning up GCP client")
		for _, client := range clientList {
			client.Close()
		}
	}

	log.Println("DEBUG: GCP client initialized successfully")

	// Populate your LiveGCPClient struct (replace with your actual implementation)
	liveGCPClient := &LiveGCPClient{
		parentString:           parentString,
		projectClient:          projectClient,
		assetClient:            assetClient,
		cloudBillingClient:     cloudBillingClient,
		serviceUsageClient:     serviceUsageClient,
		computeClient:          computeClient,
		networksClient:         networksClient,
		firewallsClient:        firewallsClient,
		zoneOperationsClient:   zoneOperationsClient,
		globalOperationsClient: globalOperationsClient,
		regionOperationsClient: regionOperationsClient,
	}

	return liveGCPClient, cleanup, nil
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

	// Create service account after project creation
	serviceAccount, err := c.CreateServiceAccount(ctx, project.ProjectId, "andaime-sa")
	if err != nil {
		l.Errorf("Failed to create service account: %v", err)
		return "", fmt.Errorf("create service account: %v", err)
	}

	// Generate a key for the service account
	serviceAccountKey, err := c.CreateServiceAccountKey(
		ctx,
		project.ProjectId,
		serviceAccount.Email,
	)
	if err != nil {
		l.Errorf("Failed to create service account key: %v", err)
		return "", fmt.Errorf("create service account key: %v", err)
	}

	// Store the service account email and key in the config
	viper.Set("gcp.service_account_email", serviceAccount.Email)
	viper.Set("gcp.service_account_key", serviceAccountKey.PrivateKeyData)
	if err := viper.WriteConfig(); err != nil {
		l.Errorf("Failed to write config: %v", err)
		return "", fmt.Errorf("write config: %v", err)
	}

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
	l := logger.Get()
	l.Infof("Enabling API %s for project %s", apiName, projectID)

	serviceName := fmt.Sprintf("projects/%s/services/%s", projectID, apiName)
	_, err := c.serviceUsageClient.EnableService(ctx, &serviceusagepb.EnableServiceRequest{
		Name: serviceName,
	})
	if err != nil {
		return fmt.Errorf("failed to enable API %s: %v", apiName, err)
	}

	l.Infof("Successfully enabled API: %s", apiName)
	return nil
}

func (c *LiveGCPClient) CreateVPCNetwork(ctx context.Context, networkName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID

	l.Infof("Enabling Compute Engine API for project: %s", projectID)
	err := c.EnableAPI(ctx, projectID, "compute.googleapis.com")
	if err != nil {
		return fmt.Errorf("failed to enable Compute Engine API: %v", err)
	}

	// Define exponential backoff
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute

	operation := func() error {
		network := &computepb.Network{
			Name:                  &networkName,
			AutoCreateSubnetworks: to.Ptr(true), // This creates a subnet mode network
			RoutingConfig: &computepb.NetworkRoutingConfig{
				RoutingMode: to.Ptr("GLOBAL"),
			},
		}

		op, err := c.networksClient.Insert(ctx, &computepb.InsertNetworkRequest{
			Project:         projectID,
			NetworkResource: network,
		})
		if err != nil {
			return fmt.Errorf("failed to create network: %v", err)
		}

		err = c.waitForGlobalOperation(ctx, projectID, op.Name())
		if err != nil {
			return fmt.Errorf("failed to wait for VPC network creation: %v", err)
		}

		return nil
	}

	err = backoff.Retry(operation, b)
	if err != nil {
		return fmt.Errorf("failed to create VPC network after retries: %v", err)
	}

	l.Infof("VPC network %s created successfully", networkName)
	return nil
}

func (c *LiveGCPClient) CreateFirewallRules(ctx context.Context, networkName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID
	l.Debugf("Creating firewall rules in project: %s", projectID)

	// Get allowed ports from the configuration
	allowedPorts := viper.GetIntSlice("gcp.allowed_ports")
	if len(allowedPorts) == 0 {
		return fmt.Errorf("no allowed ports specified in the configuration")
	}

	var allowedPortsStr []string
	for _, port := range allowedPorts {
		allowedPortsStr = append(allowedPortsStr, strconv.Itoa(port))
	}

	// Create a firewall rule to allow incoming traffic on the specified ports
	firewallRule := &computepb.Firewall{
		Name:    to.Ptr(fmt.Sprintf("%s-allow-incoming", networkName)),
		Network: to.Ptr(fmt.Sprintf("projects/%s/global/networks/%s", projectID, networkName)),
		Allowed: []*computepb.Allowed{
			{
				IPProtocol: to.Ptr("tcp"),
				Ports:      allowedPortsStr,
			},
		},
		SourceRanges: []string{"0.0.0.0/0"}, // Allow from any source IP
		Direction:    to.Ptr("INGRESS"),
	}

	op, err := c.firewallsClient.Insert(ctx, &computepb.InsertFirewallRequest{
		Project:          projectID,
		FirewallResource: firewallRule,
	})
	if err != nil {
		return fmt.Errorf("failed to create firewall rule: %v", err)
	}

	err = c.waitForGlobalOperation(ctx, projectID, op.Name())
	if err != nil {
		return fmt.Errorf("failed to wait for firewall rule creation: %v", err)
	}

	l.Infof("Firewall rules created successfully for network: %s", networkName)
	return nil
}

func (c *LiveGCPClient) CreateStorageBucket(ctx context.Context, bucketName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID
	l.Debugf("Creating storage bucket %s in project: %s", bucketName, projectID)

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer storageClient.Close()

	bucket := storageClient.Bucket(bucketName)

	// Check if the bucket already exists
	_, err = bucket.Attrs(ctx)
	if err == nil {
		l.Infof("Bucket %s already exists", bucketName)
		return nil
	}
	if err != storage.ErrBucketNotExist {
		return fmt.Errorf("failed to check bucket existence: %v", err)
	}

	// Bucket doesn't exist, so create it
	err = bucket.Create(ctx, projectID, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage bucket: %v", err)
	}

	l.Infof("Storage bucket %s created successfully", bucketName)
	return nil
}

func (c *LiveGCPClient) CreateVM(
	ctx context.Context,
	vmConfig map[string]string,
) (string, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return "", fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.ProjectID
	l.Debugf("Creating VM in project: %s", projectID)

	// Ensure the necessary APIs are enabled
	if err := c.EnableAPI(ctx, projectID, "compute.googleapis.com"); err != nil {
		return "", fmt.Errorf("failed to enable Compute Engine API: %v", err)
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
	network, err := c.getOrCreateNetwork(ctx, projectID, networkName)
	if err != nil {
		return "", fmt.Errorf("failed to get or create network: %v", err)
	}

	// Convert defaultDiskSizeGB to int64
	defaultDiskSizeGBInt, err := strconv.ParseInt(defaultDiskSizeGB, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to convert default disk size to int64: %v", err)
	}

	instance := &computepb.Instance{
		Name: &vmName,
		MachineType: to.Ptr(fmt.Sprintf(
			"zones/%s/machineTypes/%s",
			defaultZone,
			defaultMachineType,
		)),
		Disks: []*computepb.AttachedDisk{
			{
				AutoDelete: to.Ptr(true),
				Boot:       to.Ptr(true),
				Type:       to.Ptr("PERSISTENT"),
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					DiskSizeGb:  to.Ptr(defaultDiskSizeGBInt),
					SourceImage: to.Ptr(defaultSourceImage),
				},
			},
		},
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				Network: network.SelfLink,
				AccessConfigs: []*computepb.AccessConfig{
					{
						Type: to.Ptr("ONE_TO_ONE_NAT"),
						Name: to.Ptr("External NAT"),
					},
				},
			},
		},
		ServiceAccounts: []*computepb.ServiceAccount{
			{
				Email: to.Ptr("default"),
				Scopes: []string{
					"https://www.googleapis.com/auth/compute",
				},
			},
		},
	}

	req := &computepb.InsertInstanceRequest{
		Project:          projectID,
		Zone:             defaultZone,
		InstanceResource: instance,
	}

	// Create the VM instance
	op, err := c.computeClient.Insert(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create VM instance: %v", err)
	}

	// Wait for the operation to complete
	err = c.waitForOperation(ctx, projectID, defaultZone, op.Name())
	if err != nil {
		return "", fmt.Errorf("failed to wait for VM creation: %v", err)
	}

	return vmName, nil
}

func (c *LiveGCPClient) getOrCreateNetwork(
	ctx context.Context,
	projectID, networkName string,
) (*computepb.Network, error) {
	network, err := c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
		Project: projectID,
		Network: networkName,
	})
	if err == nil {
		return network, nil
	}

	if !isNotFoundError(err) {
		return nil, fmt.Errorf("failed to get network: %v", err)
	}

	// Network doesn't exist, create it
	network = &computepb.Network{
		Name:                  &networkName,
		AutoCreateSubnetworks: to.Ptr(true),
	}

	req := &computepb.InsertNetworkRequest{
		Project:         projectID,
		NetworkResource: network,
	}

	op, err := c.networksClient.Insert(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %v", err)
	}

	err = c.waitForGlobalOperation(ctx, projectID, op.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to wait for network creation: %v", err)
	}

	return c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
		Project: projectID,
		Network: networkName,
	})
}

func (c *LiveGCPClient) waitForGlobalOperation(
	ctx context.Context,
	project, operation string,
) error {
	for {
		op, err := c.globalOperationsClient.Get(ctx, &computepb.GetGlobalOperationRequest{
			Project:   project,
			Operation: operation,
		})
		if err != nil {
			return fmt.Errorf("failed to get operation status: %v", err)
		}
		if *op.Status == computepb.Operation_DONE {
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

func (c *LiveGCPClient) waitForOperation(
	ctx context.Context,
	project, zone, operation string,
) error {
	for {
		op, err := c.zoneOperationsClient.Get(ctx, &computepb.GetZoneOperationRequest{
			Project:   project,
			Zone:      zone,
			Operation: operation,
		})
		if err != nil {
			return fmt.Errorf("failed to get operation status: %v", err)
		}

		if *op.Status == computepb.Operation_DONE {
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

func (c *LiveGCPClient) SetBillingAccount(
	ctx context.Context,
	projectID, billingAccountID string,
) error {
	l := logger.Get()
	l.Infof("Setting billing account %s for project %s", billingAccountID, projectID)

	billingAccountName := getBillingAccountName(billingAccountID)

	req := &billingpb.UpdateProjectBillingInfoRequest{
		Name: fmt.Sprintf("projects/%s", projectID),
		ProjectBillingInfo: &billingpb.ProjectBillingInfo{
			ProjectId:          projectID,
			BillingAccountName: billingAccountName,
		},
	}

	_, err := c.cloudBillingClient.UpdateProjectBillingInfo(ctx, req)
	if err != nil {
		l.Errorf("Failed to update billing info: %v", err)
		return fmt.Errorf("failed to update billing info: %v", err)
	}

	return nil
}

func getBillingAccountName(billingAccountID string) string {
	if strings.HasPrefix(billingAccountID, "billingAccounts/") {
		return billingAccountID
	}
	return fmt.Sprintf("billingAccounts/%s", billingAccountID)
}

func (c *LiveGCPClient) ListBillingAccounts(ctx context.Context) ([]string, error) {
	l := logger.Get()
	l.Debug("Listing billing accounts")

	req := &billingpb.ListBillingAccountsRequest{}
	it := c.cloudBillingClient.ListBillingAccounts(ctx, req)

	var billingAccounts []string
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			l.Errorf("Failed to list billing accounts: %v", err)
			return nil, fmt.Errorf("failed to list billing accounts: %v", err)
		}
		billingAccounts = append(billingAccounts, resp.Name)
	}

	return billingAccounts, nil
}

func (c *LiveGCPClient) CreateServiceAccount(
	ctx context.Context,
	projectID string,
	saName string,
) (*iam.ServiceAccount, error) {
	l := logger.Get()
	l.Infof("Ensuring service account exists for project %s", projectID)

	service, err := iam.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam.NewService: %w", err)
	}

	request := &iam.CreateServiceAccountRequest{
		AccountId: fmt.Sprintf("%s-%s", projectID, saName),
	}
	createAccountCall := service.Projects.ServiceAccounts.Create(
		"projects/"+projectID,
		request,
	)

	account, err := createAccountCall.Do()
	if err != nil {
		// Check if the error is due to the account already existing
		if strings.Contains(err.Error(), "already exists") {
			// Get the existing account and return it
			account, err := c.iamService.Projects.ServiceAccounts.Get("projects/" + projectID + "/serviceAccounts/" + saName).
				Do()
			if err != nil {
				return nil, fmt.Errorf("failed to get existing service account: %w", err)
			}
			return account, nil
		}
		return nil, fmt.Errorf("createAccountCall.Do: %w", err)
	}

	return account, nil
}

func (c *LiveGCPClient) CreateServiceAccountKey(
	ctx context.Context,
	projectID, serviceAccountEmail string,
) (*iam.ServiceAccountKey, error) {
	l := logger.Get()
	l.Infof("Creating service account key for %s in project %s", serviceAccountEmail, projectID)

	key, err := c.iamService.Projects.ServiceAccounts.Keys.Create(
		fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, serviceAccountEmail),
		&iam.CreateServiceAccountKeyRequest{},
	).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create service account key: %v", err)
	}

	return key, nil
}

func (c *LiveGCPClient) waitForRegionalOperation(
	ctx context.Context,
	project, region, operation string,
) error {
	for {
		op, err := c.regionOperationsClient.Get(ctx, &computepb.GetRegionOperationRequest{
			Project:   project,
			Region:    region,
			Operation: operation,
		})
		if err != nil {
			return fmt.Errorf("failed to get operation status: %v", err)
		}

		if *op.Status == computepb.Operation_DONE {
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
	return strings.Contains(err.Error(), "notFound")
}

func (c *LiveGCPClient) IsAPIEnabled(
	ctx context.Context,
	projectID, apiName string,
) (bool, error) {
	l := logger.Get()
	l.Infof("Checking if API %s is enabled for project %s", apiName, projectID)

	if projectID == "" {
		return false, fmt.Errorf("project ID is empty")
	}

	serviceName := fmt.Sprintf("projects/%s/services/%s", projectID, apiName)
	service, err := c.serviceUsageClient.GetService(ctx, &serviceusagepb.GetServiceRequest{
		Name: serviceName,
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if API is enabled: %v", err)
	}

	return service.State == serviceusagepb.State_ENABLED, nil
}

func (c *LiveGCPClient) GetVMExternalIP(
	ctx context.Context,
	projectID, zone, vmName string,
) (string, error) {
	l := logger.Get()
	l.Infof("Getting external IP address for VM %s in project %s", vmName, projectID)

	req := &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     zone,
		Instance: vmName,
	}

	instance, err := c.computeClient.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %v", err)
	}

	return *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP, nil
}
