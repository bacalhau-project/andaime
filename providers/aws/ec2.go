package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers"
)

// ubuntuAMICache stores the latest Ubuntu AMI ID for each region.
var ubuntuAMICache = make(map[string]string)
var cacheLock = &sync.Mutex{}

func (p *AWSProvider) GetLatestUbuntuImage(ctx context.Context, region string) (string, error) {
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

	result, err := p.EC2Client.DescribeImages(ctx, input)
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

	amiID := *latestImage.ImageId
	cacheLock.Lock()
	ubuntuAMICache[region] = amiID
	cacheLock.Unlock()

	return amiID, nil
}

func stringPtr(s string) *string {
	return &s
}
