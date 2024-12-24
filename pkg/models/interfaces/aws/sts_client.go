package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// STSClienter interface for AWS STS operations
type STSClienter interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}
