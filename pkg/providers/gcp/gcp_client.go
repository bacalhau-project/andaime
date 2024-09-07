package gcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"math/rand"
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
	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
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

const maximumProjectIDLength = 18
const maximumUniqueProjectIDLength = 26

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

	CheckAuthentication(ctx context.Context) error
	EnableAPI(ctx context.Context, projectID, apiName string) error
	CreateVPCNetwork(ctx context.Context, networkName string) error
	CreateFirewallRules(ctx context.Context, networkName string) error
	CreateStorageBucket(ctx context.Context, bucketName string) error
	CreateComputeInstance(
		ctx context.Context,
		instanceName string,
	) (*computepb.Instance, error)
	waitForOperation(
		ctx context.Context,
		project, zone, operation string,
	) error
	SetBillingAccount(ctx context.Context, projectID, billingAccountID string) error
	ListBillingAccounts(ctx context.Context) ([]string, error)
	CreateServiceAccount(
		ctx context.Context,
		projectID string,
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
	getVMZone(
		ctx context.Context,
		projectID, vmName string,
	) (string, error)
	checkFirewallRuleExists(
		ctx context.Context,
		projectID, ruleName string,
	) error
	ValidateMachineType(ctx context.Context, machineType, location string) (bool, error)
	EnsureVPCNetwork(ctx context.Context, networkName string) error
	EnsureFirewallRules(ctx context.Context, networkName string) error
	EnsureStorageBucket(ctx context.Context, location, bucketName string) error
}

type LiveGCPClient struct {
	parentString           string
	projectClient          *resourcemanager.ProjectsClient
	assetClient            *asset.Client
	cloudBillingClient     *billing.CloudBillingClient
	iamService             *iam.Service
	serviceUsageClient     *serviceusage.Client
	storageClient          *storage.Client
	computeClient          *compute.InstancesClient
	networksClient         *compute.NetworksClient
	firewallsClient        *compute.FirewallsClient
	zoneOperationsClient   *compute.ZoneOperationsClient
	globalOperationsClient *compute.GlobalOperationsClient
	regionOperationsClient *compute.RegionOperationsClient
	zonesListClient        *compute.ZonesClient
	machineTypeListClient  *compute.MachineTypesClient

	apisEnabled chan bool
}

func (c *LiveGCPClient) EnsureVPCNetwork(ctx context.Context, networkName string) error {
	// Check if the network already exists
	_, err := c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
		Project: c.parentString,
		Network: networkName,
	})
	if err == nil {
		// Network already exists
		return nil
	}

	// If the network doesn't exist, create it
	op, err := c.networksClient.Insert(ctx, &computepb.InsertNetworkRequest{
		Project: c.parentString,
		NetworkResource: &computepb.Network{
			Name:                  &networkName,
			AutoCreateSubnetworks: to.Ptr(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create VPC network: %v", err)
	}

	opName := op.Name()
	err = c.waitForGlobalOperation(ctx, c.parentString, opName)
	if err != nil {
		return fmt.Errorf("failed to wait for VPC network creation: %v", err)
	}

	return nil
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

	iamClient, err := iam.NewService(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	// iamClient doesn't need to be closed so we don't add it to the clientList

	parentString := fmt.Sprintf("organizations/%s", organizationID)

	zonesClient, err := compute.NewZonesRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, zonesClient)

	machineTypeListClient, err := compute.NewMachineTypesRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, machineTypeListClient)

	storageClient, err := storage.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, storageClient)

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
		iamService:             iamClient,
		serviceUsageClient:     serviceUsageClient,
		computeClient:          computeClient,
		networksClient:         networksClient,
		firewallsClient:        firewallsClient,
		zoneOperationsClient:   zoneOperationsClient,
		globalOperationsClient: globalOperationsClient,
		regionOperationsClient: regionOperationsClient,
		zonesListClient:        zonesClient,
		machineTypeListClient:  machineTypeListClient,
		storageClient:          storageClient,
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

	var uniqueProjectID string
	if projectID != "" {
		uniqueProjectID = projectID
	} else {
		// Check if the project ID is too long
		if len(projectID) > maximumProjectIDLength {
			l.Warnf(
				"project ID is too long, it should be less than %d characters -- %s...",
				maximumProjectIDLength,
				projectID[:maximumProjectIDLength],
			)
			return "", fmt.Errorf(
				"project ID is too long, it should be less than %d characters",
				maximumProjectIDLength,
			)
		}

		projectPrefix := viper.GetString("general.project_prefix")
		if projectPrefix == "" {
			return "", fmt.Errorf("project prefix is not set")
		}

		timestamp := time.Now().Format("01021504") // mmddhhmm
		uniqueProjectID = fmt.Sprintf("%s-%s", projectPrefix, timestamp)

		// Check if the unique project ID is too long
		if len(uniqueProjectID) > maximumUniqueProjectIDLength {
			l.Warnf(
				"unique project ID is too long, it should be less than %d characters -- %s...",
				maximumUniqueProjectIDLength,
				uniqueProjectID[:maximumUniqueProjectIDLength],
			)
			return "", fmt.Errorf(
				"unique project ID is too long, it should be less than %d characters",
				maximumUniqueProjectIDLength,
			)
		}
	}

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

	l.Infof("Creating project: %s ...", uniqueProjectID)
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

	l.Infof(
		"Created project: %s (Display Name: %s)",
		project.ProjectId,
		project.DisplayName,
	)
	// Set the billing account for the project
	billingAccountID := viper.GetString("gcp.billing_account_id")
	if err := c.SetBillingAccount(ctx, project.ProjectId, billingAccountID); err != nil {
		return "", fmt.Errorf("failed to set billing account: %v", err)
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

	retryBackoff := backoff.NewExponentialBackOff()
	retryBackoff.MaxElapsedTime = 5 * time.Minute

	resourceTicker := time.NewTicker(ResourcePollingInterval)
	defer resourceTicker.Stop()

	allResourcesProvisioned := false

	for {
		select {
		case <-resourceTicker.C:
			if m.Quitting {
				l.Debug("Quitting detected, stopping resource polling")
				return nil
			}

			// Check if the project ID is set
			if m.Deployment.GCP.ProjectID == "" {
				l.Debug("Project ID is not set, waiting for it to be populated")
				err := backoff.Retry(func() error {
					if m.Deployment.GCP.ProjectID != "" {
						l.Debug("Project ID is now set")
						return nil
					}
					return fmt.Errorf("project ID not set")
				}, retryBackoff)

				if err != nil {
					l.Debug("Max wait time reached, stopping retry")
					return fmt.Errorf("project ID not set within maximum wait time")
				}
			}

			// Query all resources in the project
			resources, err := c.ListAllAssetsInProject(ctx, m.Deployment.GCP.ProjectID)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
				continue
			}

			l.Debugf("Poll: Found %d resources", len(resources))

			allResourcesProvisioned = true
			for _, resource := range resources {
				resourceType := resource.GetAssetType()
				resourceName := resource.GetName()

				l.Debugf("Resource: %s (Type: %s)", resourceName, resourceType)

				// Update the resource state in the deployment model
				if err := c.UpdateResourceState(resourceName, resourceType, models.ResourceStateSucceeded); err != nil {
					l.Errorf("Failed to update resource state: %v", err)
					allResourcesProvisioned = false
				}
			}

			// Check if all machines are complete
			allMachinesComplete := c.allMachinesComplete(m)

			if allResourcesProvisioned && allMachinesComplete {
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

	assetTypes := []string{
		"compute.googleapis.com/instance",
		"compute.googleapis.com/disk",
		"compute.googleapis.com/image",
		"compute.googleapis.com/network",
		// "compute.googleapis.com/Subnetwork",
		"compute.googleapis.com/firewall",
		// "iam.googleapis.com/ServiceAccount",
		// "iam.googleapis.com/Policy",
	}

	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("projects/%s", projectID),
		AssetTypes: assetTypes,
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

		// Prefix the query with the chain of thought reasoning
		l.Debugf(
			"Querying resource: %s (Type: %s)",
			resource.GetName(),
			resource.GetAssetType(),
		)

		resourceAsset := &assetpb.Asset{
			Name:       resource.GetName(),
			UpdateTime: resource.GetUpdateTime(),
			AssetType:  resource.GetAssetType(),
		}
		resources = append(resources, resourceAsset)
	}

	// Print out total resources found every 100 queries
	//nolint:gosec,mnd
	if rand.Int31n(100) < 10 {
		l.Debugf("Found %d resources", len(resources))
	}

	return resources, nil
}

// UpdateResourceState updates the state of a resource in the deployment
func (c *LiveGCPClient) UpdateResourceState(
	resourceName, resourceType string,
	state models.ResourceState,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	foundResource := false
	// Find the machine that owns this resource
	for _, machine := range m.Deployment.Machines {
		if strings.Contains(strings.ToLower(resourceName),
			strings.ToLower(machine.Name)) {
			if machine.GetResourceState(resourceType) < state {
				machine.SetResourceState(resourceType, state)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.Name,
					models.GetGCPResourceType(resourceType),
					state,
					resourceType+" deployed.",
				))
			}
			return nil
		} else if strings.Contains(strings.ToLower(resourceName), "/global/") {
			foundResource = true
			if machine.GetResourceState(resourceType) < state {
				machine.SetResourceState(resourceType, state)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.Name,
					models.GetGCPResourceType(resourceType),
					state,
					resourceType+" deployed.",
				))
			}
		}
	}

	if foundResource {
		return nil
	}

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

	// Enable the Compute Engine API
	if err := c.EnableAPI(ctx, projectID, "compute.googleapis.com"); err != nil {
		return fmt.Errorf("failed to enable Compute Engine API: %v", err)
	}

	// Get allowed ports from the configuration
	allowedPorts := viper.GetIntSlice("gcp.allowed_ports")
	if len(allowedPorts) == 0 {
		return fmt.Errorf("no allowed ports specified in the configuration")
	}

	// Use the "default" network
	networkName = "default"

	// Create a firewall rule for each allowed port
	for _, port := range allowedPorts {
		for _, machine := range m.Deployment.Machines {
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeFirewall,
				models.ResourceStatePending,
				fmt.Sprintf("Creating FW for port %d", port),
			))
		}

		ruleName := fmt.Sprintf("default-allow-%d", port)

		// Define exponential backoff
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 5 * time.Minute

		operation := func() error {
			firewallRule := &computepb.Firewall{
				Name: &ruleName,
				Network: to.Ptr(
					fmt.Sprintf("projects/%s/global/networks/%s", projectID, networkName),
				),
				Allowed: []*computepb.Allowed{
					{
						IPProtocol: to.Ptr("tcp"),
						Ports:      []string{strconv.Itoa(port)},
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
				if strings.Contains(err.Error(), "already exists") {
					l.Debugf("Firewall rule %s already exists, skipping creation", ruleName)
					return nil // Treat as success
				}
				if strings.Contains(err.Error(), "Compute Engine API has not been used") {
					l.Debugf("Compute Engine API is not yet active. Retrying... (FW Rules)")
					return err // This will trigger a retry
				}
				return backoff.Permanent(fmt.Errorf("failed to create firewall rule: %v", err))
			}

			err = c.waitForGlobalOperation(ctx, projectID, op.Name())
			if err != nil {
				return fmt.Errorf("failed to wait for firewall rule creation: %v", err)
			}

			return nil
		}

		err := backoff.Retry(operation, b)
		if err != nil {
			l.Errorf("Failed to create firewall rule for port %d after retries: %v", port, err)
			return fmt.Errorf(
				"failed to create firewall rule for port %d after retries: %v",
				port,
				err,
			)
		}
		l.Infof("Firewall rule created or already exists for port %d", port)

		for _, machine := range m.Deployment.Machines {
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeFirewall,
				models.ResourceStateRunning,
				fmt.Sprintf("Created or verified FW Rule for port %d", port),
			))
		}
	}

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

func (c *LiveGCPClient) CreateComputeInstance(
	ctx context.Context,
	instanceName string,
) (*computepb.Instance, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return nil, fmt.Errorf("global model or deployment is nil")
	}

	machine := m.Deployment.Machines[instanceName]

	projectID := m.Deployment.ProjectID
	l.Debugf("Creating VM in project: %s", projectID)

	// Ensure the necessary APIs are enabled
	if err := c.EnableAPI(ctx, projectID, "compute.googleapis.com"); err != nil {
		return nil, fmt.Errorf("failed to enable Compute Engine API: %v", err)
	}

	// Get the zone from the vmConfig
	zone := machine.Location

	// Validate the zone
	if err := c.validateZone(ctx, projectID, zone); err != nil {
		return nil, fmt.Errorf("invalid zone: %v", err)
	}

	// Create or get the network
	networkName := "default" // Use the default network
	network, err := c.getOrCreateNetwork(ctx, projectID, networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create network: %v", err)
	}

	// Get the SSH user from the deployment model
	sshUser := machine.SSHUser

	if sshUser == "" {
		return nil, fmt.Errorf("SSH user is not set in the deployment model")
	}

	publicKeyMaterial := m.Deployment.SSHPublicKeyMaterial
	if publicKeyMaterial == "" {
		return nil, fmt.Errorf("public key material is not set in the deployment model")
	}

	publicKeyMaterialB64 := base64.StdEncoding.EncodeToString([]byte(publicKeyMaterial))

	startupScriptTemplate, err := internal_gcp.GetStartupScript()
	if err != nil {
		return nil, fmt.Errorf("failed to get startup script: %v", err)
	}

	tmpl, err := template.New("startup_script").Parse(startupScriptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse startup script template: %w", err)
	}

	var scriptBuffer bytes.Buffer
	err = tmpl.Execute(&scriptBuffer, map[string]interface{}{
		"SSHUser":              sshUser,
		"PublicKeyMaterialB64": publicKeyMaterialB64,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute startup script template: %w", err)
	}

	if machine.VMSize == "" {
		return nil, fmt.Errorf("vm size is not set on this machine")
	}

	if machine.DiskSizeGB == 0 {
		return nil, fmt.Errorf("disk size is not set on this machine")
	}

	instance := &computepb.Instance{
		Name: &instanceName,
		MachineType: to.Ptr(fmt.Sprintf(
			"zones/%s/machineTypes/%s",
			zone, // Use the provided zone
			machine.VMSize,
		)),
		Disks: []*computepb.AttachedDisk{
			{
				AutoDelete: to.Ptr(true),
				Boot:       to.Ptr(true),
				Type:       to.Ptr("PERSISTENT"),
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					DiskSizeGb:  to.Ptr(int64(machine.DiskSizeGB)),
					SourceImage: to.Ptr(machine.DiskImageURL),
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
		Metadata: &computepb.Metadata{
			Items: []*computepb.Items{
				{
					Key:   to.Ptr("startup-script"),
					Value: to.Ptr(scriptBuffer.String()),
				},
			},
		},
	}

	req := &computepb.InsertInstanceRequest{
		Project:          projectID,
		Zone:             zone,
		InstanceResource: instance,
	}

	// Create the VM instance
	op, err := c.computeClient.Insert(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM instance: %v", err)
	}

	// Wait for the operation to complete
	err = c.waitForOperation(ctx, projectID, zone, op.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to wait for VM creation: %v", err)
	}

	instance, err = c.computeClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     zone,
		Instance: instanceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VM instance: %v", err)
	}

	return instance, nil
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
) (*iam.ServiceAccount, error) {
	l := logger.Get()
	l.Infof("Ensuring service account exists for project %s", projectID)

	service, err := iam.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam.NewService: %w", err)
	}

	// Create a service account name that is a combination of the project ID and the saName
	serviceAccountName := fmt.Sprintf("%s-%s", projectID, "sa")

	// Check if the service account name is within the allowed length
	if len(serviceAccountName) < 6 || len(serviceAccountName) > 30 {
		return nil, fmt.Errorf(
			"service account name is too long or too short, it should be between 6 and 30 characters",
		)
	}

	// First, try to get the existing service account
	existingSA, err := c.iamService.Projects.
		ServiceAccounts.Get(fmt.Sprintf(
		"projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com",
		projectID,
		serviceAccountName,
		projectID,
	)).
		Do()
	if err != nil {
		// If the service account does not exist, create a new one
		if isNotFoundError(err) {
			l.Infof("Service account %s does not exist, creating a new one", serviceAccountName)
			request := &iam.CreateServiceAccountRequest{
				AccountId: serviceAccountName,
			}
			createAccountCall := service.Projects.ServiceAccounts.Create(
				"projects/"+projectID,
				request,
			)

			account, err := createAccountCall.Do()
			if err != nil {
				return nil, fmt.Errorf("createAccountCall.Do: %w", err)
			}
			return account, nil
		}
		return nil, fmt.Errorf("failed to get existing service account: %w", err)
	}

	return existingSA, nil
}

func (c *LiveGCPClient) CreateServiceAccountKey(
	ctx context.Context,
	projectID, serviceAccountEmail string,
) (*iam.ServiceAccountKey, error) {
	l := logger.Get()
	l.Infof("Creating service account key for %s in project %s", serviceAccountEmail, projectID)

	// Check if the service account exists
	_, err := c.iamService.Projects.ServiceAccounts.Get("projects/" + projectID + "/serviceAccounts/" + serviceAccountEmail).
		Do()
	if err != nil {
		if isNotFoundError(err) {
			l.Infof("Service account %s does not exist, creating a new one", serviceAccountEmail)
			serviceAccount, err := c.CreateServiceAccount(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("failed to create service account: %v", err)
			}
			serviceAccountEmail = serviceAccount.Email
		} else {
			return nil, fmt.Errorf("failed to get service account: %v", err)
		}
	}

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
	l.Infof(
		"Getting external IP address for VM %s in project %s and zone %s",
		vmName,
		projectID,
		zone,
	)

	req := &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     zone, // Pass the zone value
		Instance: vmName,
	}

	instance, err := c.computeClient.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %v", err)
	}

	return *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP, nil
}

func (c *LiveGCPClient) getVMZone(
	ctx context.Context,
	projectID, vmName string,
) (string, error) {
	l := logger.Get()
	l.Debugf("Getting zone for VM %s in project %s", vmName, projectID)

	// Get the VM instance
	instance, err := c.computeClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  projectID,
		Instance: vmName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get VM instance: %v", err)
	}

	// Extract the zone from the VM instance
	zone := instance.Zone
	if zone == nil {
		return "", fmt.Errorf("zone not found for VM instance %s", vmName)
	}

	// Remove the "zones/" prefix from the zone string
	zoneStr := strings.TrimPrefix(*zone, "zones/")

	l.Debugf("Found zone %s for VM %s", zoneStr, vmName)
	return zoneStr, nil
}

func (c *LiveGCPClient) validateZone(ctx context.Context, projectID, zone string) error {
	l := logger.Get()
	l.Debugf("Validating zone: %s", zone)

	gotZone, err := c.zonesListClient.Get(ctx, &computepb.GetZoneRequest{
		Project: projectID,
		Zone:    zone,
	})
	if err != nil {
		return fmt.Errorf("failed to get zone: %v", err)
	}
	if gotZone.Name != nil && *gotZone.Name == zone {
		return nil
	}

	return fmt.Errorf("zone %s not found", zone)
}

func (c *LiveGCPClient) checkFirewallRuleExists(
	ctx context.Context,
	projectID, ruleName string,
) error {
	l := logger.Get()
	l.Debugf("Checking if firewall rule %s exists in project %s", ruleName, projectID)

	req := &computepb.GetFirewallRequest{
		Project:  projectID,
		Firewall: ruleName,
	}

	_, err := c.firewallsClient.Get(ctx, req)
	if err != nil {
		if isNotFoundError(err) {
			return fmt.Errorf("firewall rule %s does not exist", ruleName)
		}
		return fmt.Errorf("failed to check firewall rule existence: %v", err)
	}

	return nil
}

func (c *LiveGCPClient) ValidateMachineType(
	ctx context.Context,
	machineType, location string,
) (bool, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return false, fmt.Errorf("global model or deployment is nil")
	}
	projectID := m.Deployment.ProjectID
	l.Debugf("Validating machine type %s in location %s", machineType, location)

	req := &computepb.GetMachineTypeRequest{
		Project:     projectID,
		Zone:        location,
		MachineType: machineType,
	}

	_, err := c.machineTypeListClient.Get(ctx, req)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to validate machine type: %v", err)
	}

	return true, nil
}

func (c *LiveGCPClient) EnsureFirewallRules(
	ctx context.Context,
	networkName string,
) error {
	l := logger.Get()
	l.Debugf("Ensuring firewall rules for network %s", networkName)

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	projectID := m.Deployment.ProjectID

	network, err := c.getOrCreateNetwork(ctx, projectID, networkName)
	if err != nil {
		return fmt.Errorf("failed to get or create network: %v", err)
	}

	// Create a new firewall rule for each port in the deployment model
	for i, port := range m.Deployment.AllowedPorts {
		firewallRuleName := fmt.Sprintf("allow-%d", port)
		firewallRule := &computepb.Firewall{
			Name:    to.Ptr(firewallRuleName),
			Network: network.SelfLink,
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: to.Ptr("tcp"),
					Ports:      []string{strconv.Itoa(port)},
				},
			},
			Direction: to.Ptr("INGRESS"),
			Priority:  to.Ptr(int32(1000 + i)),
		}

		req := &computepb.InsertFirewallRequest{
			Project:          projectID,
			FirewallResource: firewallRule,
		}

		op, err := c.firewallsClient.Insert(ctx, req)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				l.Debugf("Firewall rule %s already exists, skipping creation", firewallRuleName)
				continue
			}
			return fmt.Errorf("failed to create firewall rule: %v", err)
		}

		l.Debugf("Firewall rule %s created, waiting for operation to complete", firewallRuleName)

		err = c.waitForGlobalOperation(ctx, projectID, op.Name())
		if err != nil {
			return fmt.Errorf("failed to wait for firewall rule creation: %v", err)
		}
	}

	return nil
}

func (c *LiveGCPClient) EnsureStorageBucket(
	ctx context.Context,
	location string,
	bucketName string,
) error {
	l := logger.Get()
	l.Debugf("Ensuring storage bucket %s exists in location %s", bucketName, location)

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	projectID := m.Deployment.ProjectID

	if c.storageClient == nil {
		return fmt.Errorf("storage client is nil")
	}

	bucket := c.storageClient.Bucket(bucketName)
	if bucket == nil {
		return fmt.Errorf("failed to create bucket handle")
	}

	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		if err == storage.ErrBucketNotExist {
			l.Debugf("Bucket %s does not exist, creating it", bucketName)
			return c.createBucket(ctx, projectID, bucketName, location)
		}
		return fmt.Errorf("failed to check if bucket exists: %v", err)
	}

	l.Debugf("Bucket %s already exists in location %s", bucketName, attrs.Location)
	return nil
}

func (c *LiveGCPClient) createBucket(
	ctx context.Context,
	projectID, bucketName, location string,
) error {
	l := logger.Get()
	l.Debugf("Creating bucket %s in location %s", bucketName, location)

	attrs := &storage.BucketAttrs{
		Location:          location,
		StorageClass:      "STANDARD",
		VersioningEnabled: true,
	}

	err := c.storageClient.Bucket(bucketName).Create(ctx, projectID, attrs)
	if err != nil {
		return fmt.Errorf("failed to create bucket: %v", err)
	}

	l.Debugf("Successfully created bucket %s", bucketName)
	return nil
}
