package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers"
)

type AWSProvider struct {
	client EC2Client
}

func NewAWSProvider() *AWSProvider {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		// In a real-world scenario, we might want to handle this error more gracefully
		panic(err)
	}
	client := ec2.NewFromConfig(cfg)
	return &AWSProvider{client: client}
}

func (p *AWSProvider) GetLatestUbuntuImage(ctx context.Context, region string) (string, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical's AWS account ID
		Filters: []ec2types.Filter{
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
	latestImage := result.Images[0]
	for _, image := range result.Images[1:] {
		if image.CreationDate != nil && latestImage.CreationDate != nil && *image.CreationDate > *latestImage.CreationDate {
			latestImage = image
		}
	}

	return *latestImage.ImageId, nil
}

func stringPtr(s string) *string {
	return &s
}
