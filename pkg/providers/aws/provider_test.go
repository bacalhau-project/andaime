package awsprovider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	mocks "github.com/bacalhau-project/andaime/mocks/aws"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewAWSProvider(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestCreateDeployment(t *testing.T) {
	mockEC2Client := new(mocks.MockEC2Clienter)
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
					ImageId:      aws.String("ami-1234567890abcdef0"),
					CreationDate: aws.String("2024-02-01T00:00:00Z"),
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
	err := provider.CreateDeployment(ctx)
	assert.NoError(t, err)

	mockEC2Client.AssertExpectations(t)
}

func TestListDeployments(t *testing.T) {
	mockEC2Client := new(mocks.MockEC2Clienter)
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
	mockEC2Client := new(mocks.MockEC2Clienter)
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
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	ctx := context.Background()

	image, err := provider.GetLatestUbuntuImage(ctx, "us-west-2")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestValidateMachineType(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	ctx := context.Background()

	valid, err := provider.ValidateMachineType(ctx, "us-west-2", "t3.micro")
	assert.NoError(t, err)
	assert.True(t, valid)

	invalid, err := provider.ValidateMachineType(ctx, "BAD_REGION", "t3.large")
	assert.Error(t, err, "Was expecting error for bad region")
	assert.False(t, invalid)

	invalid, err = provider.ValidateMachineType(ctx, "us-west-2", "BAD_MACHINE_TYPE")
	assert.Error(t, err, "Was expecting error for bad machine type")
	assert.False(t, invalid)
}

func TestGetVMExternalIP(t *testing.T) {
	mockEC2Client := new(mocks.MockEC2Clienter)
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
