package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type AWSProvider struct {
	EC2Client EC2Client
}

type EC2Client interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

type AWSProviderInterface interface {
	// AWS Infra information
	GetAllAWSRegions() ([]string, error)

	// AWS EC2 functions
	GetLatestUbuntuImage(ctx context.Context, region string) (string, error)
}

// NewEC2Client creates a new EC2 client
func NewEC2Client(ctx context.Context) (EC2Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return ec2.NewFromConfig(cfg), nil
}

// AWSProviderFactory is a function type that returns an AWSProviderInterface
type AWSProviderFactory func(ctx context.Context) (AWSProviderInterface, error)

// NewAWSProviderFunc is a variable holding the function that instantiates a new AWSProvider.
// By default, it points to a function that creates a new EC2 client and returns a new AWSProvider instance.
var NewAWSProviderFunc AWSProviderFactory = func(ctx context.Context) (AWSProviderInterface, error) {
	client, err := NewEC2Client(ctx)
	if err != nil {
		return nil, err
	}
	return &AWSProvider{
		EC2Client: client,
	}, nil
}
