package azure

import (
	"context"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

func ConvertFromStringToResourceState(state string) (string, error) {
	switch strings.ToLower(state) {
	case "succeeded", "failed", "provisioning":
		return state, nil
	}

	return "not started", nil
}

type Pollerer interface {
	PollUntilDone(
		ctx context.Context,
		options *runtime.PollUntilDoneOptions,
	) (armresources.DeploymentsClientCreateOrUpdateResponse, error)
	ResumeToken() (string, error)
	Result(ctx context.Context) (armresources.DeploymentsClientCreateOrUpdateResponse, error)
	Done() bool
	Poll(ctx context.Context) (*http.Response, error)
}
