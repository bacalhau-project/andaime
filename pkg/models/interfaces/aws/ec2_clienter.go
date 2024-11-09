package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

//go:generate mockery --name EC2Clienter
type EC2Clienter interface {
	RunInstances(
		ctx context.Context,
		params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.RunInstancesOutput, error)
	DescribeInstances(
		ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeInstancesOutput, error)
	TerminateInstances(
		ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.TerminateInstancesOutput, error)
	DescribeImages(
		ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeImagesOutput, error)
	CreateSubnet(
		ctx context.Context,
		params *ec2.CreateSubnetInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateSubnetOutput, error)
	CreateVPC(
		ctx context.Context,
		params *ec2.CreateVpcInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateVpcOutput, error)
	DescribeVPCs(
		ctx context.Context,
		params *ec2.DescribeVpcsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeVpcsOutput, error)
	CreateInternetGateway(
		ctx context.Context,
		params *ec2.CreateInternetGatewayInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateInternetGatewayOutput, error)
	AttachInternetGateway(
		ctx context.Context,
		params *ec2.AttachInternetGatewayInput,
		optFns ...func(*ec2.Options),
	) (*ec2.AttachInternetGatewayOutput, error)
	CreateRouteTable(
		ctx context.Context,
		params *ec2.CreateRouteTableInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateRouteTableOutput, error)
	CreateRoute(
		ctx context.Context,
		params *ec2.CreateRouteInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateRouteOutput, error)
	AssociateRouteTable(
		ctx context.Context,
		params *ec2.AssociateRouteTableInput,
		optFns ...func(*ec2.Options),
	) (*ec2.AssociateRouteTableOutput, error)
	DescribeAvailabilityZones(
		ctx context.Context,
		params *ec2.DescribeAvailabilityZonesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeAvailabilityZonesOutput, error)
}
