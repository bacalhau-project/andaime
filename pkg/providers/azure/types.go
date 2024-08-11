package azure

import (
	"context"
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

func ConvertFromStringToResourceState(state string) (ResourceState, error) {
	switch strings.ToLower(state) {
	case "succeeded":
		return StateSucceeded, nil
	case "failed":
		return StateFailed, nil
	case "provisioning":
		return StateProvisioning, nil
	}

	return StateNotStarted, nil
}
