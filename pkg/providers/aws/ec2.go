package aws_provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// NewEC2Client creates a new EC2 client
func NewEC2Client(ctx context.Context) (EC2Clienter, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return ec2.NewFromConfig(cfg), nil
}
