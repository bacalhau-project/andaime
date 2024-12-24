package aws

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/bacalhau-project/andaime/internal/testdata"
	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	ssh_utils "github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
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

	// Mock VPC creation
	mockEC2Client.On("CreateVpc", mock.Anything, mock.MatchedBy(func(input *ec2.CreateVpcInput) bool {
		return input.CidrBlock != nil
	})).Return(&ec2.CreateVpcOutput{
		Vpc: &types.Vpc{
			VpcId: aws.String("vpc-test123"),
			State: types.VpcStateAvailable,
		},
	}, nil)

	// Mock Internet Gateway creation
	mockEC2Client.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{
				InternetGatewayId: aws.String("igw-test123"),
			},
		}, nil)

	// Mock Security Group creation
	mockEC2Client.On("CreateSecurityGroup", mock.Anything, mock.MatchedBy(func(input *ec2.CreateSecurityGroupInput) bool {
		return input.GroupName != nil && input.VpcId != nil
	})).Return(&ec2.CreateSecurityGroupOutput{
		GroupId: aws.String("sg-test123"),
	}, nil)

	// Mock Security Group rule authorization
	mockEC2Client.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.Anything).
		Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)

	// Mock Internet Gateway attachment
	mockEC2Client.On("AttachInternetGateway", mock.Anything, mock.MatchedBy(func(input *ec2.AttachInternetGatewayInput) bool {
		return input.InternetGatewayId != nil && input.VpcId != nil
	})).Return(&ec2.AttachInternetGatewayOutput{}, nil)

	// Mock subnet creation
	mockEC2Client.On("CreateSubnet", mock.Anything, mock.MatchedBy(func(input *ec2.CreateSubnetInput) bool {
		return input.VpcId != nil && input.CidrBlock != nil
	})).Return(&ec2.CreateSubnetOutput{
		Subnet: &types.Subnet{
			SubnetId: aws.String("subnet-test123"),
			State:    types.SubnetStateAvailable,
		},
	}, nil)

	// Mock route table creation
	mockEC2Client.On("CreateRouteTable", mock.Anything, mock.MatchedBy(func(input *ec2.CreateRouteTableInput) bool {
		return input.VpcId != nil
	})).Return(&ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{
			RouteTableId: aws.String("rtb-test123"),
		},
	}, nil)

	// Mock route creation
	mockEC2Client.On("CreateRoute", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteOutput{}, nil)

	// Mock route table association
	mockEC2Client.On("AssociateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.AssociateRouteTableOutput{
			AssociationId: aws.String("rtbassoc-test123"),
		}, nil)

	// Mock DescribeRouteTables for network connectivity check
	mockEC2Client.On("DescribeRouteTables", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeRouteTablesInput) bool {
		return len(input.Filters) > 0 && input.Filters[0].Values[0] == "vpc-test123"
	})).Return(&ec2.DescribeRouteTablesOutput{
		RouteTables: []types.RouteTable{
			{
				RouteTableId: aws.String("rtb-test123"),
				VpcId:       aws.String("vpc-test123"),
				Routes: []types.Route{
					{
						DestinationCidrBlock: aws.String("0.0.0.0/0"),
						GatewayId:           aws.String("igw-test123"),
						State:               types.RouteStateActive,
					},
				},
			},
		},
	}, nil)

	// Mock DescribeVpcs for network connectivity check
	mockEC2Client.On("DescribeVpcs", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeVpcsInput) bool {
		return len(input.VpcIds) > 0 && input.VpcIds[0] == "vpc-test123"
	})).Return(&ec2.DescribeVpcsOutput{
		Vpcs: []types.Vpc{
			{
				VpcId: aws.String("vpc-test123"),
				State: types.VpcStateAvailable,
			},
		},
	}, nil)

	// Mock DescribeInternetGateways for network connectivity check
	mockEC2Client.On("DescribeInternetGateways", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeInternetGatewaysInput) bool {
		return len(input.Filters) > 0
	})).Return(&ec2.DescribeInternetGatewaysOutput{
		InternetGateways: []types.InternetGateway{
			{
				InternetGatewayId: aws.String("igw-test123"),
				Attachments: []types.InternetGatewayAttachment{
					{
						State: types.AttachmentStatusAttached,
						VpcId: aws.String("vpc-test123"),
					},
				},
			},
		},
	}, nil)

	// Mock DescribeImages for AMI lookup
	mockEC2Client.On("DescribeImages", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeImagesInput) bool {
		return len(input.Filters) == 2 &&
			*input.Filters[0].Name == "name" &&
			input.Filters[0].Values[0] == "amzn2-ami-hvm-*-x86_64-gp2" &&
			*input.Filters[1].Name == "state" &&
			input.Filters[1].Values[0] == "available" &&
			len(input.Owners) == 1 &&
			input.Owners[0] == "amazon"
	})).Return(&ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId: aws.String("ami-test123"),
				Name:    aws.String("amzn2-ami-hvm-2.0.20231218.0-x86_64-gp2"),
				State:   types.ImageStateAvailable,
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
	// Create mock STS client
	mockSTSClient := &aws_mock.MockSTSClienter{}
	mockSTSClient.On("GetCallerIdentity", mock.Anything, mock.Anything).Return(
		&sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:user/test"),
			UserId:  aws.String("AIDATEST"),
		}, nil)

	// Create SSH client mock
	var mockSSHClient *aws_mock.MockSSHClient
	mockSSHClient = &aws_mock.MockSSHClient{}
	mockSSHClient.ConnectFunc = func() (sshutils.SSHClienter, error) {
		return mockSSHClient, nil
	}
	mockSSHClient.ExecuteCommandFunc = func(ctx context.Context, command string) (string, error) { return "", nil }
	mockSSHClient.IsConnectedFunc = func() bool { return true }
	mockSSHClient.CloseFunc = func() error { return nil }
	mockSSHClient.GetClientFunc = func() *ssh.Client { return nil }
	mockSSHClient.NewSessionFunc = func() (sshutils.SSHSessioner, error) { return nil, nil }
	originalNewAWSProviderFunc := aws_provider.NewAWSProviderFunc
	defer func() {
		aws_provider.NewAWSProviderFunc = originalNewAWSProviderFunc
	}()

	// Mock NewAWSProviderFunc to return our provider with mock clients
	aws_provider.NewAWSProviderFunc = func(accountID string) (*aws_provider.AWSProvider, error) {
		// Create provider with mock clients directly, bypassing AWS credential loading
		cfg := aws.Config{Region: "us-east-1"} // Create config struct
		provider := &aws_provider.AWSProvider{
			AccountID:       accountID,
			Config:         &cfg, // Use pointer to config
			EC2Client:      mockEC2Client,
			STSClient:      mockSTSClient,
			ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAWS),
			UpdateQueue:     make(chan display.UpdateAction, aws_provider.UpdateQueueSize),
		}
		deployer := provider.GetClusterDeployer()
		deployer.SetSSHClient(mockSSHClient)
		return provider, nil
	}

	// Create deployment command once for all test cases
	cmd := GetAwsCreateDeploymentCmd()

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
			privateKeyPath := filepath.Join(tempDir, "id_rsa")
			publicKeyPath := filepath.Join(tempDir, "id_rsa.pub")

			// Create temporary SSH key files
			err := os.WriteFile(privateKeyPath, []byte(testdata.TestPrivateSSHKeyMaterial), 0600)
			require.NoError(t, err)
			err = os.WriteFile(publicKeyPath, []byte(testdata.TestPublicSSHKeyMaterial), 0644)
			require.NoError(t, err)

			// Override SSHKeyReader with MockSSHKeyReader during test
			originalSSHKeyReader := ssh_utils.SSHKeyReader
			defer func() {
				ssh_utils.SSHKeyReader = originalSSHKeyReader
			}()
			ssh_utils.SSHKeyReader = ssh_utils.MockSSHKeyReader
			// Write test configuration to file
			config := map[string]interface{}{
				"aws": map[string]interface{}{
					"account_id":             "123456789012",
					"region":                 "us-east-1",
					"machines":               tc.machines,
					"default_count_per_zone": 1,
					"default_machine_type":   "t2.micro",
					"default_disk_size_gb":   20,
				},
				"general": map[string]interface{}{
					"ssh_user":             "testuser",
					"ssh_public_key_path":  publicKeyPath,
					"ssh_private_key_path": privateKeyPath,
				},
			}

			configData, err := yaml.Marshal(config)
			require.NoError(t, err)
			err = os.WriteFile(configFile, configData, 0644)
			require.NoError(t, err)

			// Reset viper configuration
			viper.Reset()
			viper.SetConfigFile(configFile)
			err = viper.ReadInConfig()
			require.NoError(t, err)

			// Set config flag
			cmd.Flags().Set("config", configFile)

			// Execute command
			err = cmd.Execute()
			require.NoError(t, err)

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
