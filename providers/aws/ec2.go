package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers"
)

// ubuntuAMICache stores the latest Ubuntu AMI ID for each region.
var ubuntuAMICache = make(map[string]*types.Image)
var cacheLock = &sync.Mutex{}

// NewEC2Client creates a new EC2 client
func NewEC2Client(ctx context.Context) (*ec2.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Assuming ec2.NewFromConfig returns a *ec2.Client that implements EC2Client interface
	return ec2.NewFromConfig(cfg), nil
}
func (p *AWSProvider) GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error) {
	cacheLock.Lock()
	if ami, ok := ubuntuAMICache[region]; ok {
		cacheLock.Unlock()
		return ami, nil
	}
	cacheLock.Unlock()

	input := &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical's AWS account ID
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
			{
				Name:   aws.String("root-device-type"),
				Values: []string{"ebs"},
			},
			{
				Name:   aws.String("virtualization-type"),
				Values: []string{"hvm"},
			},
		},
	}

	result, err := p.EC2Client.DescribeImages(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Images) == 0 {
		return nil, providers.ErrNoImagesFound
	}

	// Sort images by creation date
	latestImage := result.Images[0]
	for _, image := range result.Images[1:] {
		if image.CreationDate != nil && latestImage.CreationDate != nil && *image.CreationDate > *latestImage.CreationDate {
			latestImage = image
		}
	}

	cacheLock.Lock()
	ubuntuAMICache[region] = &latestImage
	cacheLock.Unlock()

	return &latestImage, nil
}
