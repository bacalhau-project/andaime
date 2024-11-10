//go:build integration
// +build integration

package awsprovider

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationCreateAndDestroyInfrastructure(t *testing.T) {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

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
