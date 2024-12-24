package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
)

// STSClient implements the STSClienter interface
type STSClient struct {
	client *sts.Client
}

// NewSTSClient creates a new STSClient
func NewSTSClient(client *sts.Client) aws_interface.STSClienter {
	return &STSClient{
		client: client,
	}
}

// GetCallerIdentity gets the caller identity from AWS STS
func (c *STSClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return c.client.GetCallerIdentity(ctx, params)
}