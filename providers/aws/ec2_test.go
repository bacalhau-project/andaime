package aws

import (
	"context"
	"testing"

	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers"
)

// MockEC2Client is a mock of the EC2Client interface for testing
type MockEC2Client struct {
	DescribeImagesFunc func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

func (m *MockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return m.DescribeImagesFunc(ctx, params, optFns...)
}

func TestGetLatestUbuntuImage(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *ec2.DescribeImagesOutput
		expectedAMI   string
		expectedError error
	}{
		{
			name: "Successful retrieval",
			mockResponse: &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{ImageId: stringPtr("ami-12345678"), CreationDate: stringPtr("2023-04-01T00:00:00Z")},
					{ImageId: stringPtr("ami-87654321"), CreationDate: stringPtr("2023-03-01T00:00:00Z")},
				},
			},
			expectedAMI:   "ami-12345678",
			expectedError: nil,
		},
		{
			name:          "No images found",
			mockResponse:  &ec2.DescribeImagesOutput{Images: []types.Image{}},
			expectedAMI:   "",
			expectedError: providers.ErrNoImagesFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockEC2Client{
				DescribeImagesFunc: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return tt.mockResponse, nil
				},
			}

			provider := NewAWSProvider(mockClient)
			ami, err := provider.GetLatestUbuntuImage(context.Background(), "us-west-2")

			if ami != tt.expectedAMI {
				t.Errorf("Expected AMI %s, got %s", tt.expectedAMI, ami)
			}

			if (err != nil) != (tt.expectedError != nil) {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
		{
			name: "DescribeImages error",
			mockResponse: &ec2.DescribeImagesOutput{
				Images: []types.Image{},
			},
			expectedAMI:   "",
			expectedError: errors.New("DescribeImages error"),
		},
		{
			name: "Images in unexpected order",
			mockResponse: &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{ImageId: stringPtr("ami-87654321"), CreationDate: stringPtr("2023-03-01T00:00:00Z")},
					{ImageId: stringPtr("ami-12345678"), CreationDate: stringPtr("2023-04-01T00:00:00Z")},
				},
			},
			expectedAMI:   "ami-12345678",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockEC2Client{
				DescribeImagesFunc: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					if tt.name == "DescribeImages error" {
						return nil, errors.New("DescribeImages error")
					}
					return tt.mockResponse, nil
				},
			}

			provider := NewAWSProvider(mockClient)
			ami, err := provider.GetLatestUbuntuImage(context.Background(), "us-west-2")

			if ami != tt.expectedAMI {
				t.Errorf("Expected AMI %s, got %s", tt.expectedAMI, ami)
			}

			if (err != nil) != (tt.expectedError != nil) {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}
