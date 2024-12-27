package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	aws_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
)

// STSClient implements the STSClienter interface
type STSClient struct {
	client *sts.Client
}

// NewSTSClient creates a new STSClient
func NewSTSClient(client *sts.Client) aws_interfaces.STSClienter {
	return &STSClient{
		client: client,
	}
}

// GetCallerIdentity gets the caller identity from AWS STS
func (c *STSClient) GetCallerIdentity(
	ctx context.Context,
	params *sts.GetCallerIdentityInput,
	opts ...func(*sts.Options),
) (*sts.GetCallerIdentityOutput, error) {
	return c.client.GetCallerIdentity(ctx, params, opts...)
}
