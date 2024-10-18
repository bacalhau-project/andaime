package awsprovider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetLatestUbuntuImage(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	a, err := NewAWSProvider(viper.GetViper())
	assert.NoError(t, err)

	mockEC2Client.On("DescribeImages", mock.Anything, mock.AnythingOfType("*ec2.DescribeImagesInput"), mock.Anything).Return(&ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId:      aws.String("ami-123"),
				CreationDate: aws.String("2023-01-02T15:04:05.000Z"),
			},
		},
	}, nil)

	a.SetEC2Client(mockEC2Client)

	// Test fetching from API
	amiID, err := a.GetLatestUbuntuImage(context.Background(), "us-west-2")
	assert.NoError(t, err)
	assert.Equal(t, "ami-123", *amiID.ImageId)

	// Test fetching from cache
	amiID, err = a.GetLatestUbuntuImage(context.Background(), "us-west-2")
	assert.NoError(t, err)
	assert.Equal(t, "ami-123", *amiID.ImageId)

	mockEC2Client.AssertExpectations(t)
}

func TestGetRegion(t *testing.T) {
	tests := []struct {
		name        string
		config      *aws.Config
		expected    string
		expectError bool
	}{
		{
			name:        "Valid region",
			config:      &aws.Config{Region: "us-west-2"},
			expected:    "us-west-2",
			expectError: false,
		},
		{
			name:        "Empty region",
			config:      &aws.Config{Region: ""},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &AWSProvider{Config: tt.config}
			region, err := p.getRegion()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, region)
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name        string
		region      string
		expectError bool
	}{
		{
			name:        "Valid region",
			region:      "us-west-2",
			expectError: false,
		},
		{
			name:        "Empty region",
			region:      "",
			expectError: true,
		},
	}

	p := &AWSProvider{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateRegion(tt.region)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDescribeInstances(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{EC2Client: mockEC2Client}

	mockInstances := []types.Instance{
		{InstanceId: aws.String("i-1234567890abcdef0")},
		{InstanceId: aws.String("i-0987654321fedcba0")},
	}

	mockEC2Client.On("DescribeInstances",
		mock.Anything,
		mock.AnythingOfType("*ec2.DescribeInstancesInput"),
		mock.AnythingOfType("[]func(*ec2.Options)"),
	).Return(
		&ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{Instances: mockInstances},
			},
		},
		nil,
	)

	instances, err := provider.describeInstances(context.Background())

	assert.NoError(t, err)
	assert.Len(t, instances, 2)
	assert.Equal(t, "i-1234567890abcdef0", *instances[0].InstanceId)
	assert.Equal(t, "i-0987654321fedcba0", *instances[1].InstanceId)

	mockEC2Client.AssertExpectations(t)
}

func TestListDeployments(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{EC2Client: mockEC2Client}

	mockInstances := []types.Instance{
		{InstanceId: aws.String("i-1234567890abcdef0")},
		{InstanceId: aws.String("i-0987654321fedcba0")},
	}

	mockEC2Client.On("DescribeInstances",
		mock.Anything,
		mock.AnythingOfType("*ec2.DescribeInstancesInput"),
		mock.AnythingOfType("[]func(*ec2.Options)"),
	).Return(
		&ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{Instances: mockInstances},
			},
		},
		nil,
	)

	instances, err := provider.ListDeployments(context.Background())

	assert.NoError(t, err)
	assert.Len(t, instances, 2)
	assert.Equal(t, "i-1234567890abcdef0", *instances[0].InstanceId)
	assert.Equal(t, "i-0987654321fedcba0", *instances[1].InstanceId)

	mockEC2Client.AssertExpectations(t)
}

func TestTerminateInstances(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{EC2Client: mockEC2Client}

	instanceIDs := []string{"i-1234567890abcdef0", "i-0987654321fedcba0"}

	mockEC2Client.On("TerminateInstances",
		mock.Anything,
		mock.MatchedBy(func(input *ec2.TerminateInstancesInput) bool {
			return len(input.InstanceIds) == 2 &&
				input.InstanceIds[0] == "i-1234567890abcdef0" &&
				input.InstanceIds[1] == "i-0987654321fedcba0"
		}),
		mock.AnythingOfType("[]func(*ec2.Options)"),
	).Return(&ec2.TerminateInstancesOutput{}, nil)

	err := provider.terminateInstances(context.Background(), instanceIDs)

	assert.NoError(t, err)
	mockEC2Client.AssertExpectations(t)
}

func TestDestroyDeployment(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := &AWSProvider{EC2Client: mockEC2Client}

	mockInstances := []types.Instance{
		{InstanceId: aws.String("i-1234567890abcdef0")},
		{InstanceId: aws.String("i-0987654321fedcba0")},
	}

	mockEC2Client.On("DescribeInstances",
		mock.Anything,
		mock.AnythingOfType("*ec2.DescribeInstancesInput"),
		mock.AnythingOfType("[]func(*ec2.Options)"),
	).Return(
		&ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{Instances: mockInstances},
			},
		},
		nil,
	)

	mockEC2Client.On("TerminateInstances",
		mock.Anything,
		mock.MatchedBy(func(input *ec2.TerminateInstancesInput) bool {
			return len(input.InstanceIds) == 2 &&
				input.InstanceIds[0] == "i-1234567890abcdef0" &&
				input.InstanceIds[1] == "i-0987654321fedcba0"
		}),
		mock.AnythingOfType("[]func(*ec2.Options)"),
	).Return(&ec2.TerminateInstancesOutput{}, nil)

	err := provider.DestroyDeployment(context.Background())

	assert.NoError(t, err)
	mockEC2Client.AssertExpectations(t)
}
package aws

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewAWSProvider(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestCreateDeployment(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	err = provider.CreateDeployment(context.Background(), EC2Instance)
	assert.NoError(t, err)

	err = provider.CreateDeployment(context.Background(), SpotInstance)
	assert.NoError(t, err)
}

func TestTerminateDeployment(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	err = provider.TerminateDeployment(context.Background())
	assert.NoError(t, err)
}

func TestListInstances(t *testing.T) {
	v := viper.New()
	v.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(v)
	assert.NoError(t, err)

	instances, err := provider.ListInstances(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, instances)
}
