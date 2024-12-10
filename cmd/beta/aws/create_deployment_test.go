package aws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	sshutils_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWriteVPCIDToConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create test configuration
	config := []byte(`
deployments:
  test-deployment:
    provider: "aws"
    aws:
      account_id: "test-account-id"
      regions:
        us-west-2:
          vpc_id: ""
          security_group_id: ""
`)

	err := os.WriteFile(configFile, config, 0644)
	require.NoError(t, err)

	// Initialize viper with the test config
	viper.Reset()
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Create test deployment
	deployment := &models.Deployment{
		UniqueID:       "test-deployment",
		DeploymentType: models.DeploymentTypeAWS,
		AWS: &models.AWSDeployment{
			AccountID: "test-account-id",
			RegionalResources: &models.RegionalResources{
				VPCs: map[string]*models.AWSVPC{
					"us-west-2": {
						VPCID:           "vpc-12345",
						SecurityGroupID: "sg-12345",
					},
				},
			},
		},
	}

	// Write deployment to config
	err = deployment.UpdateViperConfig()
	require.NoError(t, err)

	// Verify the configuration was written correctly
	viper.ReadInConfig() // Reload config

	// Check AWS account ID
	assert.Equal(t,
		"test-account-id",
		viper.GetString("deployments.test-deployment.aws.account_id"),
		"AWS account ID should be written to config",
	)

	// Check VPC ID
	assert.Equal(t,
		"vpc-12345",
		viper.GetString("deployments.test-deployment.aws.regions.us-west-2.vpc_id"),
		"VPC ID should be written to config",
	)

	// Check security group ID
	assert.Equal(t,
		"sg-12345",
		viper.GetString("deployments.test-deployment.aws.regions.us-west-2.security_group_id"),
		"Security group ID should be written to config",
	)
}

func TestExecuteCreateDeployment(t *testing.T) {
	// Create mock EC2 client
	mockEC2Client := new(aws_mock.MockEC2Clienter)

	// Mock DescribeAvailabilityZones response
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, &ec2.DescribeAvailabilityZonesInput{}).
		Return(
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

	// Create SSH config mock
	mockSSHConfig := new(sshutils_mock.MockSSHConfiger)

	// Set up expectations for the Connect call
	mockSSHConfig.On("Connect").Return(nil)

	// ... rest of test setup ...
}
