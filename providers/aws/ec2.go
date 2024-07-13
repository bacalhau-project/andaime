package aws

import (
	"context"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers"
)

// EC2Client interface to allow for easy mocking in tests
type EC2Client interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

// AWSProvider implements the CloudProvider interface for AWS
type AWSProvider struct {
	client EC2Client
}

// NewAWSProvider creates a new AWSProvider instance
func NewAWSProvider(client EC2Client) *AWSProvider {
	return &AWSProvider{client: client}
}

// GetLatestUbuntuImage retrieves the latest Ubuntu image ID for a given region in AWS
func (p *AWSProvider) GetLatestUbuntuImage(ctx context.Context, region string) (string, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical's AWS account ID
		Filters: []types.Filter{
			{
				Name:   stringPtr("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"},
			},
			{
				Name:   stringPtr("architecture"),
				Values: []string{"x86_64"},
			},
			{
				Name:   stringPtr("root-device-type"),
				Values: []string{"ebs"},
			},
			{
				Name:   stringPtr("virtualization-type"),
				Values: []string{"hvm"},
			},
		},
	}

	result, err := p.client.DescribeImages(ctx, input)
	if err != nil {
		return "", err
	}

	if len(result.Images) == 0 {
		return "", providers.ErrNoImagesFound
	}

	// Sort images by creation date
	sort.Slice(result.Images, func(i, j int) bool {
		return *result.Images[i].CreationDate > *result.Images[j].CreationDate
	})

	return *result.Images[0].ImageId, nil
}

func stringPtr(s string) *string {
	return &s
}
