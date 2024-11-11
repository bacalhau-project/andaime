//nolint:lll
package awsprovider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	EC2InstanceType  = "EC2"
	SpotInstanceType = "Spot"
)

type EC2Clienter interface {
	DescribeImages(
		ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeImagesOutput, error)
	CreateVpc(
		ctx context.Context,
		params *ec2.CreateVpcInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateVpcOutput, error)
	CreateSubnet(
		ctx context.Context,
		params *ec2.CreateSubnetInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateSubnetOutput, error)
	CreateSecurityGroup(
		ctx context.Context,
		params *ec2.CreateSecurityGroupInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateSecurityGroupOutput, error)
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
	DescribeVpcs(
		ctx context.Context,
		params *ec2.DescribeVpcsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(
		ctx context.Context,
		params *ec2.DescribeSubnetsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(
		ctx context.Context,
		params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeSecurityGroupsOutput, error)
	TerminateInstances(
		ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.TerminateInstancesOutput, error)
	DeleteSecurityGroup(
		ctx context.Context,
		params *ec2.DeleteSecurityGroupInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteSubnet(
		ctx context.Context,
		params *ec2.DeleteSubnetInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DeleteSubnetOutput, error)
	DeleteVpc(
		ctx context.Context,
		params *ec2.DeleteVpcInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DeleteVpcOutput, error)
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
	DescribeRouteTables(
		ctx context.Context,
		params *ec2.DescribeRouteTablesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeRouteTablesOutput, error)
}
