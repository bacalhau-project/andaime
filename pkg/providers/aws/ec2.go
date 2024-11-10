package awsprovider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	awsinterfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
)

type LiveEC2Client struct {
	client *ec2.Client
}

// NewEC2Client creates a new EC2 client
func NewEC2Client(ctx context.Context) (awsinterfaces.EC2Clienter, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &LiveEC2Client{client: ec2.NewFromConfig(cfg)}, nil
}

// CreateEC2Instance creates a new EC2 instance
func (c *LiveEC2Client) CreateEC2Instance(
	ctx context.Context,
	input *ec2.RunInstancesInput,
	options ...func(*ec2.Options),
) (*ec2.RunInstancesOutput, error) {
	return c.client.RunInstances(ctx, input, options...)
}

func (c *LiveEC2Client) RunInstances(
	ctx context.Context,
	input *ec2.RunInstancesInput,
	options ...func(*ec2.Options),
) (*ec2.RunInstancesOutput, error) {
	return c.client.RunInstances(ctx, input, options...)
}

// DescribeEC2Instances describes EC2 instances
func (c *LiveEC2Client) DescribeInstances(
	ctx context.Context,
	input *ec2.DescribeInstancesInput,
	options ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return c.client.DescribeInstances(ctx, input, options...)
}

// DescribeImages describes images
func (c *LiveEC2Client) DescribeImages(
	ctx context.Context,
	input *ec2.DescribeImagesInput,
	options ...func(*ec2.Options),
) (*ec2.DescribeImagesOutput, error) {
	return c.client.DescribeImages(ctx, input, options...)
}

// TerminateInstances terminates EC2 instances
func (c *LiveEC2Client) TerminateInstances(
	ctx context.Context,
	input *ec2.TerminateInstancesInput,
	options ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	return c.client.TerminateInstances(ctx, input, options...)
}

// CreateVpc creates a new VPC
func (c *LiveEC2Client) CreateVpc(
	ctx context.Context,
	input *ec2.CreateVpcInput,
	options ...func(*ec2.Options),
) (*ec2.CreateVpcOutput, error) {
	return c.client.CreateVpc(ctx, input, options...)
}

// DescribeVpcs describes VPCs
func (c *LiveEC2Client) DescribeVpcs(
	ctx context.Context,
	input *ec2.DescribeVpcsInput,
	options ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	return c.client.DescribeVpcs(ctx, input, options...)
}

// CreateSubnet creates a new subnet in a VPC
func (c *LiveEC2Client) CreateSubnet(
	ctx context.Context,
	input *ec2.CreateSubnetInput,
	options ...func(*ec2.Options),
) (*ec2.CreateSubnetOutput, error) {
	return c.client.CreateSubnet(ctx, input, options...)
}

// CreateInternetGateway creates a new Internet Gateway
func (c *LiveEC2Client) CreateInternetGateway(
	ctx context.Context,
	input *ec2.CreateInternetGatewayInput,
	options ...func(*ec2.Options),
) (*ec2.CreateInternetGatewayOutput, error) {
	return c.client.CreateInternetGateway(ctx, input, options...)
}

// AttachInternetGateway attaches an Internet Gateway to a VPC
func (c *LiveEC2Client) AttachInternetGateway(
	ctx context.Context,
	input *ec2.AttachInternetGatewayInput,
	options ...func(*ec2.Options),
) (*ec2.AttachInternetGatewayOutput, error) {
	return c.client.AttachInternetGateway(ctx, input, options...)
}

// CreateRouteTable creates a new route table
func (c *LiveEC2Client) CreateRouteTable(
	ctx context.Context,
	input *ec2.CreateRouteTableInput,
	options ...func(*ec2.Options),
) (*ec2.CreateRouteTableOutput, error) {
	return c.client.CreateRouteTable(ctx, input, options...)
}

// CreateRoute creates a new route in a route table
func (c *LiveEC2Client) CreateRoute(
	ctx context.Context,
	input *ec2.CreateRouteInput,
	options ...func(*ec2.Options),
) (*ec2.CreateRouteOutput, error) {
	return c.client.CreateRoute(ctx, input, options...)
}

// AssociateRouteTable associates a subnet with a route table
func (c *LiveEC2Client) AssociateRouteTable(
	ctx context.Context,
	input *ec2.AssociateRouteTableInput,
	options ...func(*ec2.Options),
) (*ec2.AssociateRouteTableOutput, error) {
	return c.client.AssociateRouteTable(ctx, input, options...)
}

// DescribeAvailabilityZones describes availability zones
func (c *LiveEC2Client) DescribeAvailabilityZones(
	ctx context.Context,
	input *ec2.DescribeAvailabilityZonesInput,
	options ...func(*ec2.Options),
) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return c.client.DescribeAvailabilityZones(ctx, input, options...)
}

var _ awsinterfaces.EC2Clienter = &LiveEC2Client{}
