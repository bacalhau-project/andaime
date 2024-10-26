package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	asset "cloud.google.com/go/asset/apiv1"
	billing "cloud.google.com/go/billing/apiv1"
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	serviceusagepb "cloud.google.com/go/serviceusage/apiv1/serviceusagepb"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/protobuf/proto"
)

const (
	maxBackOffTime     = 5 * time.Minute
	serviceAccountName = "andaime-sa"
)

type LiveGCPClient struct {
	parentString           string
	projectClient          *resourcemanager.ProjectsClient
	assetClient            *asset.Client
	billingClient          *billing.CloudBillingClient
	iamService             *iam.Service
	serviceUsageClient     *serviceusage.Client
	storageClient          *storage.Client
	computeClient          *compute.InstancesClient
	networksClient         *compute.NetworksClient
	addressesClient        *compute.AddressesClient
	firewallsClient        *compute.FirewallsClient
	operationsClient       *compute.GlobalOperationsClient
	zonesClient            *compute.ZonesClient
	machineTypesClient     *compute.MachineTypesClient
	resourceManagerService *cloudresourcemanager.Service
}

type CloseableClient interface {
	Close() error
}

var NewGCPClientFunc = NewGCPClient

func NewGCPClient(
	ctx context.Context,
	organizationID string,
) (gcp_interface.GCPClienter, func(), error) {
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

	addressesClient, err := compute.NewAddressesRESTClient(ctx, clientOpts...)
	if err != nil {
		return nil, nil, err
	}
	clientList = append(clientList, addressesClient)

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

	resourceManagerService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource manager service: %w", err)
	}

	// Cleanup function to close all clients
	cleanup := func() {
		l.Debug("Cleaning up GCP client")
		for _, client := range clientList {
			client.Close()
		}
	}

	l.Debug("GCP client initialized successfully")

	// Populate your LiveGCPClient struct (replace with your actual implementation)
	liveGCPClient := &LiveGCPClient{
		parentString:           parentString,
		projectClient:          projectClient,
		assetClient:            assetClient,
		billingClient:          cloudBillingClient,
		iamService:             iamClient,
		serviceUsageClient:     serviceUsageClient,
		computeClient:          computeClient,
		networksClient:         networksClient,
		addressesClient:        addressesClient,
		firewallsClient:        firewallsClient,
		operationsClient:       globalOperationsClient,
		zonesClient:            zonesClient,
		machineTypesClient:     machineTypeListClient,
		storageClient:          storageClient,
		resourceManagerService: resourceManagerService,
	}

	// Credential check and parent string setup (replace with your actual implementation)
	if err := liveGCPClient.checkCredentials(ctx); err != nil {
		l.Errorf("Credential check failed: %v", err)
		// Close clients on error
		for _, client := range clientList {
			client.Close()
		}
		return nil, nil, err
	}
	return liveGCPClient, cleanup, nil
}
func (c *LiveGCPClient) SetParentString(organizationID string) {
	c.parentString = fmt.Sprintf("organizations/%s", organizationID)
}

func (c *LiveGCPClient) GetParentString() string {
	return c.parentString
}

func isNotFoundError(err error) bool {
	return status.Code(err) == codes.NotFound ||
		status.Code(err) == codes.Unknown ||
		err.Error() == "rpc error: code = NotFound desc = googleapi: Error 404: Not Found, notFound"
}

func (c *LiveGCPClient) TestComputeEngineAPI(
	ctx context.Context,
	projectID string,
	elapsedTime, maxElapsedTime time.Duration,
) error {
	l := logger.Get()

	l.Infof(
		"Testing Compute Engine API for project %s: (%s/%s)",
		projectID,
		utils.FormatDuration(
			elapsedTime,
			utils.FormatMinutes|utils.FormatSeconds,
		),
		utils.FormatDuration(
			maxElapsedTime,
			utils.FormatMinutes|utils.FormatSeconds,
		),
	)

	// Test listing instances
	req := &computepb.ListInstancesRequest{
		Project: projectID,
		Zone:    "us-central1-a", // You might want to make this configurable
	}

	it := c.computeClient.List(ctx, req)
	_, err := it.Next()
	if err != nil && err != iterator.Done {
		return fmt.Errorf("failed to list instances: %v", err)
	}

	l.Infof("Successfully tested Compute Engine API for project %s", projectID)
	return nil
}

func (c *LiveGCPClient) TestCloudStorageAPI(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Testing Cloud Storage API for project %s", projectID)

	// Test listing buckets
	it := c.storageClient.Buckets(ctx, projectID)
	_, err := it.Next()
	if err != nil && err != iterator.Done {
		return fmt.Errorf("failed to list buckets: %v", err)
	}

	l.Infof("Successfully tested Cloud Storage API for project %s", projectID)
	return nil
}

func (c *LiveGCPClient) TestIAMPermissions(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Testing IAM permissions for project %s", projectID)

	// Test listing service accounts
	resource := fmt.Sprintf("projects/%s", projectID)
	_, err := c.iamService.Projects.ServiceAccounts.List(resource).Do()
	if err != nil {
		return fmt.Errorf("failed to list service accounts: %v", err)
	}

	l.Infof("Successfully tested IAM permissions for project %s", projectID)
	return nil
}

func (c *LiveGCPClient) TestServiceUsageAPI(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Testing Service Usage API for project %s", projectID)

	// Test listing enabled services
	req := &serviceusagepb.ListServicesRequest{
		Parent: fmt.Sprintf("projects/%s", projectID),
		Filter: "state:ENABLED",
	}

	it := c.serviceUsageClient.ListServices(ctx, req)
	_, err := it.Next()
	if err != nil && err != iterator.Done {
		return fmt.Errorf("failed to list enabled services: %v", err)
	}

	l.Infof("Successfully tested Service Usage API for project %s", projectID)
	return nil
}
func (c *LiveGCPClient) EnsureVPCNetwork(ctx context.Context, projectID, networkName string) error {
	l := logger.Get()
	l.Infof("Ensuring VPC network %s exists in project %s", networkName, projectID)

	getReq := &computepb.GetNetworkRequest{
		Project: projectID,
		Network: networkName,
	}

	_, err := c.networksClient.Get(ctx, getReq)
	if err == nil {
		l.Infof("VPC network %s already exists in project %s", networkName, projectID)
		return nil
	}

	if !isNotFoundError(err) {
		return fmt.Errorf("error checking for existing network: %v", err)
	}

	createReq := &computepb.InsertNetworkRequest{
		Project: projectID,
		NetworkResource: &computepb.Network{
			Name:                  proto.String(networkName),
			AutoCreateSubnetworks: proto.Bool(true),
		},
	}

	op, err := c.networksClient.Insert(ctx, createReq)
	if err != nil {
		return fmt.Errorf("error creating network: %v", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("error waiting for network creation: %v", err)
	}

	l.Infof("Successfully created VPC network %s in project %s", networkName, projectID)
	return nil
}

func (c *LiveGCPClient) EnsureServiceAccount(
	ctx context.Context,
	projectID, serviceAccountName string,
) error {
	l := logger.Get()
	l.Infof("Ensuring service account %s exists in project %s", serviceAccountName, projectID)

	fullServiceAccountName := fmt.Sprintf(
		"projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com",
		projectID,
		serviceAccountName,
		projectID,
	)

	_, err := c.iamService.Projects.ServiceAccounts.Get(fullServiceAccountName).Do()
	if err == nil {
		l.Infof("Service account %s already exists in project %s", serviceAccountName, projectID)
		return nil
	}

	if !isNotFoundError(err) {
		return fmt.Errorf("error checking for existing service account: %v", err)
	}

	createReq := &iam.CreateServiceAccountRequest{
		AccountId: serviceAccountName,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: serviceAccountName,
		},
	}

	_, err = c.iamService.Projects.ServiceAccounts.Create(fmt.Sprintf("projects/%s", projectID), createReq).
		Do()
	if err != nil {
		return fmt.Errorf("error creating service account: %v", err)
	}

	l.Infof("Successfully created service account %s in project %s", serviceAccountName, projectID)
	return nil
}

func (c *LiveGCPClient) generateStartupScript(sshUser string, publicKeyMaterial string) string {
	script := `#!/bin/bash
set -e

# Create the SSH user if it doesn't exist
if ! id "` + sshUser + `" &>/dev/null; then
    useradd -m -s /bin/bash ` + sshUser + `
fi

# Add the SSH public key to the user's authorized keys
mkdir -p /home/` + sshUser + `/.ssh
echo "` + publicKeyMaterial + `" >> /home/` + sshUser + `/.ssh/authorized_keys
chmod 600 /home/` + sshUser + `/.ssh/authorized_keys
chown -R ` + sshUser + `:` + sshUser + ` /home/` + sshUser + `/.ssh

# Add the SSH User to the sudo group
usermod -aG sudo ` + sshUser + `

# Allow SSH user to sudo without password
echo "` + sshUser + ` ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Update and install necessary packages
apt-get update
apt-get install -y docker.io

# Start Docker service
systemctl start docker
systemctl enable docker

# Additional setup steps can be added here
`
	return script
}

var _ gcp_interface.GCPClienter = &LiveGCPClient{}
