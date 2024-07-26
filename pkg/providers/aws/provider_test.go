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

	// Mock the DescribeImages call with the correct number of arguments
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

	// Verify that all expected mock calls were made
	mockEC2Client.AssertExpectations(t)
}
