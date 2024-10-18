package awsprovider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsinterfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEC2Client is a mock of EC2Clienter interface
type MockEC2Client struct {
	mock.Mock
}

func (m *MockEC2Client) RunInstances(
	ctx context.Context,
	params *ec2.RunInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.RunInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.RunInstancesOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func (m *MockEC2Client) TerminateInstances(
	ctx context.Context,
	params *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.TerminateInstancesOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeImages(
	ctx context.Context,
	params *ec2.DescribeImagesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeImagesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeImagesOutput), args.Error(1)
}

func (m *MockEC2Client) DescribeInstanceTypes(
	ctx context.Context,
	params *ec2.DescribeInstanceTypesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstanceTypesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeInstanceTypesOutput), args.Error(1)
}

func TestNewAWSProvider(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestCreateDeployment(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{
		EC2Client: mockEC2Client,
		Region:    "us-west-2",
	}

	ctx := context.Background()

	// Mock DescribeImages
	mockEC2Client.On("DescribeImages", mock.Anything, mock.Anything).
		Return(&ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      new(string),
					CreationDate: new(string),
				},
			},
		}, nil)

	// Mock RunInstances
	mockEC2Client.On("RunInstances", mock.Anything, mock.Anything).
		Return(&ec2.RunInstancesOutput{
			Instances: []types.Instance{
				{
					InstanceId: new(string),
				},
			},
		}, nil)

	// Test EC2 instance deployment
	err := provider.CreateDeployment(ctx, awsinterfaces.EC2Instance)
	assert.NoError(t, err)

	// Test Spot instance deployment
	err = provider.CreateDeployment(ctx, awsinterfaces.SpotInstance)
	assert.NoError(t, err)

	mockEC2Client.AssertExpectations(t)
}

func TestListDeployments(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{
		EC2Client: mockEC2Client,
		Region:    "us-west-2",
	}

	ctx := context.Background()

	// Mock DescribeInstances
	mockEC2Client.On("DescribeInstances", mock.Anything, mock.Anything).
		Return(&ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: new(string),
						},
					},
				},
			},
		}, nil)

	instances, err := provider.ListDeployments(ctx)
	assert.NoError(t, err)
	assert.Len(t, instances, 1)

	mockEC2Client.AssertExpectations(t)
}

func TestTerminateDeployment(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{
		EC2Client: mockEC2Client,
		Region:    "us-west-2",
	}

	ctx := context.Background()

	// Mock DescribeInstances
	mockEC2Client.On("DescribeInstances", mock.Anything, mock.Anything).
		Return(&ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: new(string),
						},
					},
				},
			},
		}, nil)

	// Mock TerminateInstances
	mockEC2Client.On("TerminateInstances", mock.Anything, mock.Anything).
		Return(&ec2.TerminateInstancesOutput{}, nil)

	err := provider.TerminateDeployment(ctx)
	assert.NoError(t, err)

	mockEC2Client.AssertExpectations(t)
}

func TestGetLatestUbuntuImage(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{
		EC2Client: mockEC2Client,
		Region:    "us-west-2",
	}

	ctx := context.Background()

	// Mock DescribeImages
	mockEC2Client.On("DescribeImages", mock.Anything, mock.Anything).
		Return(&ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      new(string),
					CreationDate: new(string),
				},
			},
		}, nil)

	image, err := provider.GetLatestUbuntuImage(ctx, "us-west-2")
	assert.NoError(t, err)
	assert.NotNil(t, image)

	mockEC2Client.AssertExpectations(t)
}

func TestValidateMachineType(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{
		EC2Client: mockEC2Client,
		Region:    "us-west-2",
	}

	ctx := context.Background()

	// Mock DescribeInstanceTypes
	mockEC2Client.On("DescribeInstanceTypes", mock.Anything, mock.Anything).
		Return(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []types.InstanceTypeInfo{
				{
					InstanceType: types.InstanceTypeT3Micro,
				},
			},
		}, nil)

	valid, err := provider.ValidateMachineType(ctx, "t3.micro")
	assert.NoError(t, err)
	assert.True(t, valid)

	mockEC2Client.AssertExpectations(t)
}

func TestGetVMExternalIP(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{
		EC2Client: mockEC2Client,
		Region:    "us-west-2",
	}

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"
	expectedIP := "203.0.113.1"

	// Mock DescribeInstances
	mockEC2Client.On("DescribeInstances", mock.Anything, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      &instanceID,
						PublicIpAddress: &expectedIP,
					},
				},
			},
		},
	}, nil)

	ip, err := provider.GetVMExternalIP(ctx, instanceID)
	assert.NoError(t, err)
	assert.Equal(t, expectedIP, ip)

	mockEC2Client.AssertExpectations(t)
}
