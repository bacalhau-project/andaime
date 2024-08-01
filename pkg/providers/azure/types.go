package azure

import (
	"context"
	"encoding/json"
	"time"

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
	Data            []AzureResource                        `json:"data"`
	Facets          []armresourcegraph.FacetClassification `json:"facets"`
	ResultTruncated *armresourcegraph.ResultTruncated      `json:"resultTruncated"`
	TotalRecords    *int64                                 `json:"totalRecords"`
}

// AzureResource represents a generic Azure resource
type AzureResource struct {
	ID                string
	Name              string
	Type              string
	Location          string
	Tags              map[string]string
	ProvisioningState string
	Properties        map[string]interface{}
	CreatedTime       time.Time
	ChangedTime       time.Time
}

// UnmarshalJSON implements custom JSON unmarshaling for AzureResource
func (r *AzureResource) UnmarshalJSON(data []byte) error {
	// Define a temporary struct to handle the basic structure
	type TempResource struct {
		ID         string            `json:"id"`
		Name       string            `json:"name"`
		Type       string            `json:"type"`
		Location   string            `json:"location"`
		Tags       map[string]string `json:"tags"`
		Properties json.RawMessage   `json:"properties"`
	}

	var temp TempResource
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Copy the basic fields
	r.ID = temp.ID
	r.Name = temp.Name
	r.Type = temp.Type
	r.Location = temp.Location
	r.Tags = temp.Tags

	// Unmarshal the properties into a map
	var props map[string]interface{}
	if err := json.Unmarshal(temp.Properties, &props); err != nil {
		return err
	}

	// Extract common fields from properties
	if state, ok := props["provisioningState"].(string); ok {
		r.ProvisioningState = state
		delete(props, "provisioningState")
	}
	if created, ok := props["createdTime"].(string); ok {
		r.CreatedTime, _ = time.Parse(time.RFC3339, created)
		delete(props, "createdTime")
	}
	if changed, ok := props["changedTime"].(string); ok {
		r.ChangedTime, _ = time.Parse(time.RFC3339, changed)
		delete(props, "changedTime")
	}

	// Store the remaining properties
	r.Properties = props

	return nil
}
