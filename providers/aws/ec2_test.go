package aws_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers/aws"
	"github.com/bacalhau-project/andaime/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEC2Client is a mock of EC2Client interface
type MockEC2Client struct {
	mock.Mock
}

func (m *MockEC2Client) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput, opts ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*ec2.DescribeImagesOutput), args.Error(1)
}

func TestGetLatestUbuntuImage(t *testing.T) {
	mockEC2Client := new(MockEC2Client)
	provider := aws.AWSProvider{EC2Client: mockEC2Client}

	mockEC2Client.On("DescribeImages", mock.Anything, mock.Anything).Return(&ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId:      utils.StringPtr("ami-123"),
				CreationDate: utils.StringPtr("2023-01-02T15:04:05.000Z"),
			},
		},
	}, nil)

	// Test fetching from API
	amiID, err := provider.GetLatestUbuntuImage(context.Background(), "us-west-2")
	assert.NoError(t, err)
	assert.Equal(t, "ami-123", amiID)

	// Test fetching from cache
	amiID, err = provider.GetLatestUbuntuImage(context.Background(), "us-west-2")
	assert.NoError(t, err)
	assert.Equal(t, "ami-123", amiID)

	mockEC2Client.AssertExpectations(t)
}
