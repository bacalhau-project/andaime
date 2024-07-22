//nolint:lll
package aws_provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

// AWSProviderFunc is a function type that returns an AWSProviderInterface
type AWSProviderFunc func(ctx context.Context) (AWSProviderer, error)

type AWSProviderer interface {
	GetConfig() *aws.Config
	SetConfig(*aws.Config)
	GetEC2Client() (EC2Clienter, error)
	SetEC2Client(EC2Clienter)

	GetLatestUbuntuImage(context.Context, string) (*types.Image, error)
}

type EC2Clienter interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	CreateVpc(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	//CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	//AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
}

// NewAWSProviderFunc is a variable holding the function that instantiates a new AWSProvider.
// By default, it points to a function that creates a new EC2 client and returns a new AWSProvider instance.
var NewAWSProviderFunc AWSProviderFunc = func(ctx context.Context) (AWSProviderer, error) {
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

var MockAWSProviderFunc AWSProviderFunc = func(ctx context.Context) (AWSProviderer, error) {
	mockAWSProvider := new(MockAWSProvider)
	mockAWSProvider.On("GetEC2Client").Return(&ec2.Client{}, nil)
	mockAWSProvider.On("GetConfig").Return(&aws.Config{})
	mockAWSProvider.On("GetLatestUbuntuImage").Return(&types.Image{}, nil)
	return mockAWSProvider, nil
}

type MockAWSProvider struct {
	mock.Mock
	Config    aws.Config
	EC2Client EC2Clienter
}

// GetConfig mocks the GetConfig method
func (m *MockAWSProvider) GetConfig() *aws.Config {
	return &aws.Config{}
}

func (m *MockAWSProvider) SetConfig(config *aws.Config) {
	m.Config = *config
}

// GetEC2Client mocks the GetEC2Client method
func (m *MockAWSProvider) GetEC2Client() (EC2Clienter, error) {
	return m.EC2Client, nil
}

// SetEC2Client mocks the SetEC2Client method
func (m *MockAWSProvider) SetEC2Client(client EC2Clienter) {
	m.EC2Client = client
}

func (m *MockAWSProvider) GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error) {
	return &types.Image{}, nil
}

type MockEC2Client struct {
	mock.Mock
}

func (m *MockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeImagesOutput), args.Error(1)
}

func (m *MockEC2Client) GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error) {
	args := m.Called(ctx)
	return args.Get(0).(*types.Image), args.Error(1)
}

func (m *MockEC2Client) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.CreateVpcOutput), args.Error(1)
}

func (m *MockEC2Client) CreateSubnet(ctx context.Context, params *ec2.CreateSubnetInput, optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.CreateSubnetOutput), args.Error(1)
}

func (m *MockEC2Client) CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.CreateSecurityGroupOutput), args.Error(1)
}

func (m *MockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.AuthorizeSecurityGroupIngressOutput), args.Error(1)
}

func (m *MockEC2Client) RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.RunInstancesOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeVpcsOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeSubnetsOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeSecurityGroupsOutput), args.Error(1)
}

func (m *MockEC2Client) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.TerminateInstancesOutput), args.Error(1)
}

func (m *MockEC2Client) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DeleteSecurityGroupOutput), args.Error(1)
}

func (m *MockEC2Client) DeleteSubnet(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DeleteSubnetOutput), args.Error(1)
}

func (m *MockEC2Client) DeleteVpc(ctx context.Context, params *ec2.DeleteVpcInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DeleteVpcOutput), args.Error(1)
}
