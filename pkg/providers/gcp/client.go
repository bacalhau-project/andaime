package gcp

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"github.com/cenkalti/backoff/v4"
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
	parentString       string
	projectClient      *resourcemanager.ProjectsClient
	assetClient        *asset.Client
	billingClient      *billing.CloudBillingClient
	iamService         *iam.Service
	serviceUsageClient *serviceusage.Client
	storageClient      *storage.Client
	computeClient      *compute.InstancesClient
	networksClient     *compute.NetworksClient
	firewallsClient    *compute.FirewallsClient
	operationsClient   *compute.GlobalOperationsClient
	zonesClient        *compute.ZonesClient
	machineTypesClient *compute.MachineTypesClient
	rmService          *cloudresourcemanager.Service
}

var NewGCPClientFunc = NewGCPClient

func NewGCPClient(
	ctx context.Context,
	organizationID string,
) (gcp_interface.GCPClienter, func(), error) {
	creds, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find default credentials: %v", err)
	}

	clientOpts := []option.ClientOption{option.WithCredentials(creds)}
	var clients []interface{}

	createClient := func(newClientFunc interface{}) (interface{}, error) {
		client, err := callNewClientFunc(ctx, newClientFunc, clientOpts...)
		if err == nil {
			clients = append(clients, client)
		}
		return client, err
	}

	gc := &LiveGCPClient{}
	gc.SetParentString(fmt.Sprintf("organizations/%s", organizationID))

	projectClient, err := createClient(resourcemanager.NewProjectsClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating project client")
	}
	gc.projectClient = projectClient.(*resourcemanager.ProjectsClient)

	assetClient, err := createClient(asset.NewClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating asset client")
	}
	gc.assetClient = assetClient.(*asset.Client)

	billingClient, err := createClient(billing.NewCloudBillingClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating billing client")
	}
	gc.billingClient = billingClient.(*billing.CloudBillingClient)

	serviceUsageClient, err := createClient(serviceusage.NewClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating service usage client")
	}
	gc.serviceUsageClient = serviceUsageClient.(*serviceusage.Client)

	computeClient, err := createClient(compute.NewInstancesRESTClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating compute client")
	}
	gc.computeClient = computeClient.(*compute.InstancesClient)

	networksClient, err := createClient(compute.NewNetworksRESTClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating networks client")
	}
	gc.networksClient = networksClient.(*compute.NetworksClient)

	firewallsClient, err := createClient(compute.NewFirewallsRESTClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating firewalls client")
	}
	gc.firewallsClient = firewallsClient.(*compute.FirewallsClient)

	operationsClient, err := createClient(compute.NewGlobalOperationsRESTClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating operations client")
	}
	gc.operationsClient = operationsClient.(*compute.GlobalOperationsClient)

	zonesClient, err := createClient(compute.NewZonesRESTClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating zones client")
	}
	gc.zonesClient = zonesClient.(*compute.ZonesClient)

	machineTypesClient, err := createClient(compute.NewMachineTypesRESTClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating machine types client")
	}
	gc.machineTypesClient = machineTypesClient.(*compute.MachineTypesClient)

	storageClient, err := createClient(storage.NewClient)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating storage client")
	}
	gc.storageClient = storageClient.(*storage.Client)

	rmService, err := createClient(cloudresourcemanager.NewService)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating resource manager service")
	}
	gc.rmService = rmService.(*cloudresourcemanager.Service)

	iamService, err := createClient(iam.NewService)
	if err != nil {
		return nil, func() {}, fmt.Errorf("Error creating IAM service")
	}
	gc.iamService = iamService.(*iam.Service)

	if err := gc.checkCredentials(ctx); err != nil {
		cleanup(clients)
		return nil, nil, err
	}

	return gc, func() { cleanup(clients) }, nil
}

func (c *LiveGCPClient) SetParentString(parentString string) {
	c.parentString = parentString
}

func (c *LiveGCPClient) GetParentString() string {
	return c.parentString
}

func cleanup(clients []interface{}) {
	for _, client := range clients {
		if c, ok := client.(interface{ Close() error }); ok {
			c.Close()
		}
	}
}

func callNewClientFunc(
	ctx context.Context,
	newClientFunc interface{},
	opts ...option.ClientOption,
) (interface{}, error) {
	results := reflect.ValueOf(newClientFunc).
		Call(append([]reflect.Value{reflect.ValueOf(ctx)}, reflect.ValueOf(opts).Interface().([]reflect.Value)...))
	if len(results) != 2 {
		return nil, fmt.Errorf("unexpected number of return values from client creation function")
	}
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

func isNotFoundError(err error) bool {
	return status.Code(err) == codes.NotFound
}

func (c *LiveGCPClient) testProjectPermissions(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Testing permissions and resource creation for project %s", projectID)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = maxBackOffTime

	return backoff.Retry(func() error {
		if err := c.TestComputeEngineAPI(ctx, projectID); err != nil {
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

func (c *LiveGCPClient) TestComputeEngineAPI(ctx context.Context, projectID string) error {
	l := logger.Get()
	l.Infof("Testing Compute Engine API for project %s", projectID)
	
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
	req := &iam.ListServiceAccountsRequest{
		Name: resource,
	}
	
	_, err := c.iamService.Projects.ServiceAccounts.List(resource).Do(req)
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
			Name:                  &networkName,
			AutoCreateSubnetworks: proto.Bool(true),
		},
	}

	op, err := c.networksClient.Insert(ctx, createReq)
	if err != nil {
		return fmt.Errorf("error creating network: %v", err)
	}

	err = c.waitForOperation(ctx, projectID, op)
	if err != nil {
		return fmt.Errorf("error waiting for network creation: %v", err)
	}

	l.Infof("Successfully created VPC network %s in project %s", networkName, projectID)
	return nil
}
func (c *LiveGCPClient) EnsureServiceAccount(ctx context.Context, projectID, serviceAccountName string) error {
	l := logger.Get()
	l.Infof("Ensuring service account %s exists in project %s", serviceAccountName, projectID)

	fullServiceAccountName := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", projectID, serviceAccountName, projectID)

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

	_, err = c.iamService.Projects.ServiceAccounts.Create(fmt.Sprintf("projects/%s", projectID), createReq).Do()
	if err != nil {
		return fmt.Errorf("error creating service account: %v", err)
	}

	l.Infof("Successfully created service account %s in project %s", serviceAccountName, projectID)
	return nil
}
func (c *LiveGCPClient) generateStartupScript() string {
	script := `#!/bin/bash
set -e

# Update and install necessary packages
apt-get update
apt-get install -y docker.io

# Start Docker service
systemctl start docker
systemctl enable docker

# Pull and run your application container
# Replace with your actual container image and run command
docker pull your-container-image:latest
docker run -d your-container-image:latest

# Additional setup steps can be added here
`
	return script
}

func (c *LiveGCPClient) waitForOperation(ctx context.Context, projectID string, op *computepb.Operation) error {
	for {
		result, err := c.operationsClient.Wait(ctx, &computepb.WaitGlobalOperationRequest{
			Project:   projectID,
			Operation: *op.Name,
		})
		if err != nil {
			return err
		}

		if result.Status == computepb.Operation_DONE {
			if result.Error != nil {
				return fmt.Errorf("operation failed: %v", result.Error)
			}
			return nil
		}

		time.Sleep(5 * time.Second)
	}
}
