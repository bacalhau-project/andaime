package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/providers"
)

type mockEC2Client struct {
	DescribeImagesFunc func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

func (m *mockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return m.DescribeImagesFunc(ctx, params, optFns...)
}

func TestGetLatestUbuntuImage(t *testing.T) {
	mockClient := &mockEC2Client{
		DescribeImagesFunc: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{
						ImageId:      stringPtr("ami-12345"),
						CreationDate: stringPtr("2023-05-01T00:00:00Z"),
					},
					{
						ImageId:      stringPtr("ami-67890"),
						CreationDate: stringPtr("2023-05-02T00:00:00Z"),
					},
				},
			}, nil
		},
	}

	provider := &AWSProvider{client: mockClient}

	amiID, err := provider.GetLatestUbuntuImage(context.Background(), "us-west-2")
	if err != nil {
		t.Fatalf("GetLatestUbuntuImage failed: %v", err)
	}

	if amiID != "ami-67890" {
		t.Errorf("Expected AMI ID ami-67890, got %s", amiID)
	}
}

func TestGetLatestUbuntuImageNoImages(t *testing.T) {
	mockClient := &mockEC2Client{
		DescribeImagesFunc: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{},
			}, nil
		},
	}

	provider := &AWSProvider{client: mockClient}

	_, err := provider.GetLatestUbuntuImage(context.Background(), "us-west-2")
	if err != providers.ErrNoImagesFound {
		t.Errorf("Expected ErrNoImagesFound, got %v", err)
	}
}
