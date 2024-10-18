//nolint:lll
package awsprovider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/bacalhau-project/andaime/pkg/logger"
	awsinterfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/spf13/viper"
)

const (
	EC2InstanceType  = "EC2"
	SpotInstanceType = "Spot"
)

// AWSProviderFunc is a function type that returns an AWSProviderer
type AWSProviderFunc func(ctx context.Context) (awsinterfaces.AWSProviderer, error)

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
	//CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	//AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
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
}

// NewAWSProviderFunc is a variable holding the function that instantiates a new AWSProvider.
// By default, it points to a function that creates a new EC2 client and returns a new AWSProvider instance.
var NewAWSProviderFunc AWSProviderFunc = func(ctx context.Context) (awsinterfaces.AWSProviderer, error) {
	log := logger.Get()
	client, err := NewEC2Client(ctx)
	if err != nil {
		return nil, err
	}
	awsProvider, err := NewAWSProvider(viper.GetViper())
	if err != nil {
		log.Fatalf("Unable to create AWS Provider: %s", err)
		return nil, err
	}
	awsProvider.SetEC2Client(client)
	return awsProvider, nil
}
