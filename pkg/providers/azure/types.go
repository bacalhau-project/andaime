package azure

import (
	"context"

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

// AzureResource represents a single Azure resource
type AzureResource struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	ID       string            `json:"id"`
	Location string            `json:"location"`
	Tags     map[string]string `json:"tags"`
}

// QueryResponse represents the response from an Azure Resource Graph query
type QueryResponse struct {
	Count           *int64                                 `json:"count"`
	Data            []AzureResource                        `json:"data"`
	Facets          []armresourcegraph.FacetClassification `json:"facets"`
	ResultTruncated *armresourcegraph.ResultTruncated      `json:"resultTruncated"`
	TotalRecords    *int64                                 `json:"totalRecords"`
}
