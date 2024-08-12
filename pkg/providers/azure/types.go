package azure

import (
	"context"
	"strings"

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
