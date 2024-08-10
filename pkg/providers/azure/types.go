package azure

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

type ResourceGraphClientAPI interface {
	Resources(
		ctx context.Context,
		request armresourcegraph.QueryRequest,
		options *armresourcegraph.ClientResourcesOptions,
	) (armresourcegraph.ClientResourcesResponse, error)
}

type AzureProviderInterface interface {
	CreateDeployment(context.Context) error
}

// QueryResponse represents the response from an Azure Resource Graph query
type QueryResponse struct {
	Count           *int64                                 `json:"count"`
	Facets          []armresourcegraph.FacetClassification `json:"facets"`
	ResultTruncated *armresourcegraph.ResultTruncated      `json:"resultTruncated"`
	TotalRecords    *int64                                 `json:"totalRecords"`
}

var ResourceTypeMap = map[string]reflect.Type{
	"microsoft.compute/virtualmachines": reflect.TypeOf((*armcompute.VirtualMachine)(nil)).
		Elem(),
	"microsoft.network/networkinterfaces": reflect.TypeOf((*armnetwork.Interface)(nil)).Elem(),
	"microsoft.network/networksecuritygroups": reflect.TypeOf((*armnetwork.SecurityGroup)(nil)).
		Elem(),
	"microsoft.network/virtualnetworks": reflect.TypeOf((*armnetwork.VirtualNetwork)(nil)).
		Elem(),
	"microsoft.network/publicipaddresses": reflect.TypeOf((*armnetwork.PublicIPAddress)(nil)).
		Elem(),
	"microsoft.compute/disks": reflect.TypeOf((*armcompute.Disk)(nil)).Elem(),
}

func UpdateResourceStatus(
	ctx context.Context,
	resourceName, resource, provisioningState string,
) error {
	stateMachine := GetGlobalStateMachine()

	var state ResourceState
	switch strings.ToLower(provisioningState) {
	case "succeeded":
		state = StateSucceeded
	case "failed":
		state = StateFailed
	default:
		state = StateProvisioning
	}

	stateMachine.UpdateStatus(
		resourceName,
		resource,
		state,
	)
	return nil
}

func ProcessResource(ctx context.Context, resourceType string, resource interface{}) error {
	resourceValue := reflect.ValueOf(resource)
	if resourceValue.Kind() == reflect.Ptr {
		resourceValue = resourceValue.Elem()
	}

	nameField := resourceValue.FieldByName("Name")
	if !nameField.IsValid() {
		return fmt.Errorf("resource does not have a Name field")
	}

	propertiesField := resourceValue.FieldByName("Properties")
	if !propertiesField.IsValid() {
		return fmt.Errorf("resource does not have a Properties field")
	}

	provisioningStateField := propertiesField.Elem().FieldByName("ProvisioningState")
	if !provisioningStateField.IsValid() {
		return fmt.Errorf("resource properties does not have a ProvisioningState field")
	}

	return UpdateResourceStatus(
		ctx,
		resourceType,
		nameField.String(),
		provisioningStateField.String(),
	)
}

func (p *AzureProvider) ProcessResources(ctx context.Context, resources interface{}) error {
	resourcesValue := reflect.ValueOf(resources)
	if resourcesValue.Kind() != reflect.Slice {
		return fmt.Errorf("resources must be a slice")
	}

	for i := 0; i < resourcesValue.Len(); i++ {
		resource := resourcesValue.Index(i).Interface()
		resourceType := reflect.TypeOf(resource).String()

		if err := ProcessResource(ctx, resourceType, resource); err != nil {
			return fmt.Errorf("failed to process resource: %w", err)
		}
	}

	return nil
}
