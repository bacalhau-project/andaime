package azure

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

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
