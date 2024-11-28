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
	CreateVpc(
		ctx context.Context,
		params *ec2.CreateVpcInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateVpcOutput, error)
	DescribeVpcs(
		ctx context.Context,
		params *ec2.DescribeVpcsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeVpcsOutput, error)
	CreateSecurityGroup(
		ctx context.Context,
		params *ec2.CreateSecurityGroupInput,
		optFns ...func(*ec2.Options),
	) (*ec2.CreateSecurityGroupOutput, error)
	DescribeSecurityGroups(
		ctx context.Context,
		params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeSecurityGroupsOutput, error)
	AuthorizeSecurityGroupIngress(
		ctx context.Context,
		params *ec2.AuthorizeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options),
	) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
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
	DescribeRouteTables(
		ctx context.Context,
		params *ec2.DescribeRouteTablesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeRouteTablesOutput, error)
	DeleteVpc(
		ctx context.Context,
		params *ec2.DeleteVpcInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DeleteVpcOutput, error)
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
}
