package aws

import (
	"fmt"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func (p *AWSProvider) GetAllAWSRegions(ctx context.Context) ([]string, error) {
	// Create an EC2 client with the provider's configuration.
	ec2Client := ec2.NewFromConfig(p.Config)

	// Call DescribeRegions to get the list of regions.
	output, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to describe regions: %w", err)
	}

	// Extract region names from the response.
	regions := make([]string, len(output.Regions))
	for i, region := range output.Regions {
		regions[i] = *region.RegionName
	}

	return regions, nil
}
