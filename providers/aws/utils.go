package aws

import (
	"fmt"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/bacalhau-project/andaime/utils"
)

func (p *AWSProvider) GetAllAWSRegions() ([]string, error) {
	// Load the AWS SDK's default configuration.
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create an EC2 client with the configuration.
	ec2Client := ec2.NewFromConfig(cfg)

	// Call DescribeRegions to get the list of regions.
	output, err := ec2Client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to describe regions: %w", err)
	}

	// Extract region names from the response.
	var regions []string
	for _, region := range output.Regions {
		regions = append(regions, *region.RegionName)
	}

	return regions, nil
}

func (p *AWSProvider) GetAWSTargetRegions() ([]string, error) {
	config, err := utils.LoadConfig("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return config.AWS.Regions, nil
}
