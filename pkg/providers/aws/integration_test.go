//go:build integration
// +build integration

package awsprovider

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
	awsmock "github.com/bacalhau-project/andaime/mocks/aws"
)

// setupMockEC2Client creates and configures a mock EC2 client for testing
func setupMockEC2Client() *awsmock.MockEC2Clienter {
	// Mock EC2 client setup
	mockEC2Client := new(awsmock.MockEC2Clienter)

	// Mock DescribeAvailabilityZones response
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).Return(
		&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
				{
					ZoneName:   aws.String("us-east-1a"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-east-1"),
					State:      types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName:   aws.String("us-east-1b"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-east-1"),
					State:      types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName:   aws.String("us-east-1c"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-east-1"),
					State:      types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName:   aws.String("us-west-1a"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-west-1"),
					State:      types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName:   aws.String("us-west-1b"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-west-1"),
					State:      types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName:   aws.String("us-west-1c"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-west-1"),
					State:      types.AvailabilityZoneStateAvailable,
				},
			},
		}, nil)

	return mockEC2Client
}

func TestIntegrationCreateAndDestroyInfrastructure(t *testing.T) {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Set the mock client
	mockEC2Client := setupMockEC2Client()
	provider.SetEC2Client(mockEC2Client)

	// Create infrastructure
	err = provider.CreateInfrastructure(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, provider.VPCID)

	// Verify VPC exists and is available
	vpcExists, err := verifyVPCExists(ctx, provider)
	assert.NoError(t, err)
	assert.True(t, vpcExists)

	// Verify network connectivity
	networkConnectivity, err := verifyNetworkConnectivity(ctx, provider)
	assert.NoError(t, err)
	assert.True(t, networkConnectivity)

	// Test cleanup
	err = provider.Destroy(ctx)
	assert.NoError(t, err)

	// Verify resources are cleaned up
	vpcExists, err = verifyVPCExists(ctx, provider)
	assert.NoError(t, err)
	assert.False(t, vpcExists)
}

func verifyVPCExists(ctx context.Context, provider *AWSProvider) (bool, error) {
	if provider.EC2Client == nil {
		return false, nil
	}

	input := &ec2.DescribeVpcsInput{
		VpcIds: []string{provider.VPCID},
	}

	result, err := provider.EC2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return false, err
	}

	return len(result.Vpcs) > 0, nil
}

func verifyNetworkConnectivity(ctx context.Context, provider *AWSProvider) (bool, error) {
	if provider.EC2Client == nil {
		return false, nil
	}

	// Verify subnets can route to internet gateway
	input := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:  aws.String("vpc-id"),
				Value: []string{provider.VPCID},
			},
		},
	}

	result, err := provider.EC2Client.DescribeRouteTables(ctx, input)
	if err != nil {
		return false, err
	}

	// Check if route table has internet gateway route
	for _, rt := range result.RouteTables {
		for _, route := range rt.Routes {
			if route.GatewayId != nil && *route.GatewayId != "" {
				return true, nil
			}
		}
	}

	return false, nil
}
