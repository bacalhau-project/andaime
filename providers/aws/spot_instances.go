package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// SpotInstanceConfig holds the configuration for creating spot instances
type SpotInstanceConfig struct {
	KeyPairName   string
	InstanceType  string
	VPCCIDRBlock  string
	VPCTagKey     string
	VPCTagValue   string
}

// CreateSpotInstancesInRegion creates spot instances in the specified AWS region
func CreateSpotInstancesInRegion(ctx context.Context, cfg aws.Config, region string, orchestrators []string, token string, instancesPerRegion int, config SpotInstanceConfig) ([]string, error) {
	client := ec2.NewFromConfig(cfg)

	// Ensure key pair exists
	if err := ensureKeyPairExists(ctx, client, config.KeyPairName); err != nil {
		return nil, fmt.Errorf("failed to ensure key pair exists: %w", err)
	}

	// Create or get VPC
	vpcID, err := createVPCIfNotExists(ctx, client, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create or get VPC: %w", err)
	}

	// Create Internet Gateway
	igwID, err := createInternetGateway(ctx, client, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Internet Gateway: %w", err)
	}

	// TODO: Implement the rest of the function
	return nil, nil
}

// ensureKeyPairExists checks if the key pair exists, and creates it if it doesn't
func ensureKeyPairExists(ctx context.Context, client *ec2.Client, keyPairName string) error {
	// Check if the key pair already exists
	_, err := client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{
		KeyNames: []string{keyPairName},
	})

	if err == nil {
		// Key pair exists
		log.Printf("Key pair '%s' already exists.", keyPairName)
		return nil
	}

	// If the error is not because the key doesn't exist, return the error
	var notFoundErr *types.InvalidKeyPairNotFound
	if !aws.IsErrorMessageContains(err, "InvalidKeyPair.NotFound") {
		return fmt.Errorf("error describing key pair: %w", err)
	}

	// TODO: Implement key pair creation logic
	return fmt.Errorf("key pair creation not implemented")
}

// createVPCIfNotExists creates a new VPC if one with the specified tag doesn't exist
func createVPCIfNotExists(ctx context.Context, client *ec2.Client, config SpotInstanceConfig) (string, error) {
	// Check if a VPC with the specified tag already exists
	describeVPCsOutput, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + config.VPCTagKey),
				Values: []string{config.VPCTagValue},
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("error describing VPCs: %w", err)
	}

	if len(describeVPCsOutput.Vpcs) > 0 {
		// VPC already exists
		vpcID := *describeVPCsOutput.Vpcs[0].VpcId
		log.Printf("VPC with tag %s:%s already exists: %s", config.VPCTagKey, config.VPCTagValue, vpcID)
		return vpcID, nil
	}

	// Create a new VPC
	createVPCOutput, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(config.VPCCIDRBlock),
	})

	if err != nil {
		return "", fmt.Errorf("error creating VPC: %w", err)
	}

	vpcID := *createVPCOutput.Vpc.VpcId

	// Tag the VPC
	_, err = client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{vpcID},
		Tags: []types.Tag{
			{
				Key:   aws.String(config.VPCTagKey),
				Value: aws.String(config.VPCTagValue),
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("error tagging VPC: %w", err)
	}

	// Enable DNS hostnames for the VPC
	_, err = client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:              aws.String(vpcID),
		EnableDnsHostnames: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})

	if err != nil {
		return "", fmt.Errorf("error enabling DNS hostnames for VPC: %w", err)
	}

	// Enable DNS support for the VPC
	_, err = client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:            aws.String(vpcID),
		EnableDnsSupport: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})

	if err != nil {
		return "", fmt.Errorf("error enabling DNS support for VPC: %w", err)
	}

	log.Printf("Created new VPC: %s", vpcID)
	return vpcID, nil
}

func createInternetGateway(ctx context.Context, client *ec2.Client, vpcID string) (string, error) {
	// Create a new Internet Gateway
	createIgwOutput, err := client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{})
	if err != nil {
		return "", fmt.Errorf("error creating Internet Gateway: %w", err)
	}

	igwID := *createIgwOutput.InternetGateway.InternetGatewayId

	// Attach the Internet Gateway to the VPC
	_, err = client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	})
	if err != nil {
		return "", fmt.Errorf("error attaching Internet Gateway to VPC: %w", err)
	}

	log.Printf("Created and attached Internet Gateway: %s", igwID)
	return igwID, nil
}

// TODO: Implement other helper functions