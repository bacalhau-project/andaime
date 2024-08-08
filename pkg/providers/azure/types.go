package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// ResourceTypeMap maps resource types to their Go struct types and slice fields in AzureResources.
var ResourceTypeMap = map[string]struct {
	resourceType reflect.Type
	sliceField   string
}{
	"Microsoft.Compute/virtualMachines": {
		reflect.TypeOf((*armcompute.VirtualMachine)(nil)).Elem(),
		"VirtualMachines",
	},
	"Microsoft.Network/networkInterfaces": {
		reflect.TypeOf((*armnetwork.Interface)(nil)).Elem(),
		"NetworkInterfaces",
	},
	"Microsoft.Network/networkSecurityGroups": {
		reflect.TypeOf((*armnetwork.SecurityGroup)(nil)).Elem(),
		"NetworkSecurityGroups",
	},
	"Microsoft.Network/virtualNetworks": {
		reflect.TypeOf((*armnetwork.VirtualNetwork)(nil)).Elem(),
		"VirtualNetworks",
	},
	"Microsoft.Network/publicIPAddresses": {
		reflect.TypeOf((*armnetwork.PublicIPAddress)(nil)).Elem(),
		"PublicIPAddresses",
	},
}

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
	Data            []AzureResources                       `json:"data"`
	Facets          []armresourcegraph.FacetClassification `json:"facets"`
	ResultTruncated *armresourcegraph.ResultTruncated      `json:"resultTruncated"`
	TotalRecords    *int64                                 `json:"totalRecords"`
}

type AzureResources struct {
	VirtualMachines       []armcompute.VirtualMachine
	NetworkInterfaces     []armnetwork.Interface
	NetworkSecurityGroups []armnetwork.SecurityGroup
	VirtualNetworks       []armnetwork.VirtualNetwork
	PublicIPAddresses     []armnetwork.PublicIPAddress
}

func GetTypedResource[T any](resource map[string]interface{}) (T, error) {
	var t T

	// Marshal the resource to JSON
	jsonData, err := json.Marshal(resource)
	if err != nil {
		return t, fmt.Errorf("failed to marshal resource to JSON: %w", err)
	}

	// Unmarshal JSON to the correct type
	err = json.Unmarshal(jsonData, &t)
	if err != nil {
		return t, fmt.Errorf("failed to unmarshal JSON to type %T: %w", t, err)
	}

	return t, nil
}
func SetTypedResource(resources *AzureResources, resource interface{}) error {
	resourceType := reflect.TypeOf(resource)
	resourceInfo, ok := ResourceTypeMap[resourceType.String()]
	if !ok {
		return fmt.Errorf("unsupported resource type: %v", resourceType)
	}

	// Get the slice field in AzureResources
	sliceField := reflect.ValueOf(resources).Elem().FieldByName(resourceInfo.sliceField)

	// Append the resource to the slice
	sliceField.Set(reflect.Append(sliceField, reflect.ValueOf(resource)))
	return nil
}

func SetTypedResourceFromInterface(
	resources *AzureResources,
	resource map[string]interface{},
) error {
	typeString, ok := resource["type"].(string)
	if !ok {
		return fmt.Errorf("resource 'type' not found or invalid")
	}
	resourceInfo, ok := ResourceTypeMap[typeString]
	if !ok {
		return fmt.Errorf("unsupported resource type: %s", typeString)
	}

	typedResource := reflect.New(resourceInfo.resourceType).Interface()
	if err := json.Unmarshal([]byte(resource["properties"].(string)), typedResource); err != nil {
		return err
	}
	return SetTypedResource(resources, typedResource)
}

func (ar *AzureResources) GetTotalResourcesCount() int {
	total := 0
	v := reflect.ValueOf(*ar)
	for i := 0; i < v.NumField(); i++ {
		if field := v.Field(i); field.Kind() == reflect.Slice {
			total += field.Len()
		}
	}
	return total
}

func (ar *AzureResources) GetAllResources() <-chan struct {
	Type     string
	Resource interface{}
} {
	resourceChannel := make(chan struct {
		Type     string
		Resource interface{}
	})

	go func() {
		defer close(resourceChannel)

		v := reflect.ValueOf(*ar)
		t := reflect.TypeOf(*ar)
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.Kind() == reflect.Slice {
				for j := 0; j < field.Len(); j++ {
					resourceChannel <- struct {
						Type     string
						Resource interface{}
					}{
						Type:     t.Field(i).Type.Elem().Name(), // Get the name of the resource type
						Resource: field.Index(j).Interface(),
					}
				}
			}
		}
	}()

	return resourceChannel
}
