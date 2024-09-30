package gcp

import (
	"context"
	"fmt"
	"reflect"
	"time"

	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	asset "cloud.google.com/go/asset/apiv1"
	billing "cloud.google.com/go/billing/apiv1"
	compute "cloud.google.com/go/compute/apiv1"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
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
