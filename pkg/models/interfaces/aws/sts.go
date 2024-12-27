package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// STSClienter defines the interface for AWS STS client operations
type STSClienter interface {
	GetCallerIdentity(
		ctx context.Context,
		params *sts.GetCallerIdentityInput,
		optFns ...func(*sts.Options),
	) (*sts.GetCallerIdentityOutput, error)
}
