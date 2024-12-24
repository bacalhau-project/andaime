package aws

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
				},
			}, nil)

	// Mock RunInstances for spot instances
	mockEC2Client.On(
		"RunInstances",
		mock.Anything,
		mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
			return input.InstanceMarketOptions != nil &&
				input.InstanceMarketOptions.MarketType == types.MarketTypeSpot
		}),
	).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: aws.String("i-spot123"),
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
			},
		},
	}, nil)

	// Mock RunInstances for on-demand instances
	mockEC2Client.On(
		"RunInstances",
		mock.Anything,
		mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
			return input.InstanceMarketOptions == nil
		}),
	).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: aws.String("i-ondemand123"),
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
			},
		},
	}, nil)

	// Mock DescribeInstances for both spot and on-demand
	mockEC2Client.On(
		"DescribeInstances",
		mock.Anything,
		mock.Anything,
	).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: aws.String("i-spot123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: aws.String("1.2.3.4"),
					},
					{
						InstanceId: aws.String("i-ondemand123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: aws.String("5.6.7.8"),
					},
				},
			},
		},
	}, nil)

	// Create SSH client mock
	mockSSHClient := &aws_provider.MockSSHClient{
		ConnectFunc: func() error { return nil },
		ExecuteCommandFunc: func(ctx context.Context, command string) (string, error) { return "", nil },
		IsConnectedFunc: func() bool { return true },
	}
	// Test both spot and on-demand configurations
	testCases := []struct {
		name     string
		machines []map[string]interface{}
		wantSpot bool
	}{
		{
			name: "spot_instance",
			machines: []map[string]interface{}{
				{
					"location": "us-east-1a",
					"parameters": map[string]interface{}{
						"count":        1,
						"type":         "t2.micro",
						"spot":         true,
						"orchestrator": true,
					},
				},
			},
			wantSpot: true,
		},
		{
			name: "on_demand_instance",
			machines: []map[string]interface{}{
				{
					"location": "us-east-1b",
					"parameters": map[string]interface{}{
						"count": 1,
						"type":  "t2.micro",
					},
				},
			},
			wantSpot: false,
		},
		{
			name: "mixed_instances",
			machines: []map[string]interface{}{
				{
					"location": "us-east-1a",
					"parameters": map[string]interface{}{
						"count":        1,
						"type":         "t2.micro",
						"spot":         true,
						"orchestrator": true,
					},
				},
				{
					"location": "us-east-1b",
					"parameters": map[string]interface{}{
						"count": 1,
						"type":  "t2.micro",
					},
				},
			},
			wantSpot: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary config file
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "config.yaml")

			// Write test configuration to file
			config := map[string]interface{}{
				"aws": map[string]interface{}{
					"account_id": "123456789012",
					"region":     "us-east-1",
					"machines":   tc.machines,
				},
				"general": map[string]interface{}{
					"ssh_user":             "testuser",
					"ssh_public_key_path":  "/tmp/test.pub",
					"ssh_private_key_path": "/tmp/test",
				},
			}

			configData, err := yaml.Marshal(config)
			require.NoError(t, err)
			err = os.WriteFile(configFile, configData, 0644)
			require.NoError(t, err)

			// Create deployment command and set config flag
			cmd := NewCreateDeploymentCmd()
			cmd.Flags().String("config", "", "Path to the configuration file")
			err = cmd.Flags().Set("config", configFile)
			require.NoError(t, err)

			// Initialize AWS provider with mock clients
			provider, err := aws_provider.NewAWSProvider("123456789012")
			require.NoError(t, err)
			provider.SetEC2Client(mockEC2Client)
			deployer := provider.GetClusterDeployer()
			deployer.SetSSHClient(mockSSHClient)

			// Set AWS provider in the command context
			ctx := context.WithValue(context.Background(), aws_provider.ProviderKey{}, provider)
			cmd.SetContext(ctx)
			// Execute command
			err = cmd.Execute()
			assert.NoError(t, err)
			// Verify spot instance configuration
			if tc.wantSpot {
				mockEC2Client.AssertCalled(t, "RunInstances", mock.Anything, mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
					return input.InstanceMarketOptions != nil &&
						input.InstanceMarketOptions.MarketType == types.MarketTypeSpot
				}))
			} else {
				mockEC2Client.AssertCalled(t, "RunInstances", mock.Anything, mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
					return input.InstanceMarketOptions == nil
				}))
			}
		})
	}
}
