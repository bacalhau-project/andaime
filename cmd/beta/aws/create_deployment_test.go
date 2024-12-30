package aws

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/bacalhau-project/andaime/internal/testutil"
	aws_mocks "github.com/bacalhau-project/andaime/mocks/aws"
	common_mocks "github.com/bacalhau-project/andaime/mocks/common"
	sshutils_mocks "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CreateDeploymentTestSuite struct {
	suite.Suite
	ctx                    context.Context
	viperConfigFile        string
	testSSHPublicKeyPath   string
	testPrivateKeyPath     string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockEC2Client          *aws_mocks.MockEC2Clienter
	mockSTSClient          *aws_mocks.MockSTSClienter
	mockSSHConfig          *sshutils_mocks.MockSSHConfiger
	mockClusterDeployer    *common_mocks.MockClusterDeployerer
	awsProvider            *aws_provider.AWSProvider
	deployment             *models.Deployment
	origGetGlobalModelFunc func() *display.DisplayModel
	origSSHConfigFunc      func(string, int, string, string) (sshutils_interfaces.SSHConfiger, error)
}

func (suite *CreateDeploymentTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	testSSHPublicKeyPath,
		cleanupPublicKey,
		testPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	suite.testSSHPublicKeyPath = testSSHPublicKeyPath
	suite.cleanupPublicKey = cleanupPublicKey
	suite.testPrivateKeyPath = testPrivateKeyPath
	suite.cleanupPrivateKey = cleanupPrivateKey

	suite.mockEC2Client = new(aws_mocks.MockEC2Clienter)
	suite.mockSTSClient = new(aws_mocks.MockSTSClienter)
	suite.mockSSHConfig = new(sshutils_mocks.MockSSHConfiger)
}
func (suite *CreateDeploymentTestSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	if suite.viperConfigFile != "" {
		_ = os.Remove(suite.viperConfigFile)
	}
}
func (suite *CreateDeploymentTestSuite) SetupTest() {
	// Reset and setup viper configuration
	viper.Reset()
	suite.setupViper()

	// Initialize all mock objects
	suite.mockEC2Client = new(aws_mocks.MockEC2Clienter)
	suite.mockSTSClient = new(aws_mocks.MockSTSClienter)
	suite.mockSSHConfig = new(sshutils_mocks.MockSSHConfiger)
	suite.mockClusterDeployer = new(common_mocks.MockClusterDeployerer)

	// Set up STS mock expectations
	suite.mockSTSClient.On("GetCallerIdentity", mock.Anything, mock.AnythingOfType("*sts.GetCallerIdentityInput")).
		Return(&sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
			UserId:  aws.String("AIDACKCEVSQ6C2EXAMPLE"),
		}, nil)

	// Initialize deployment with AWS configuration
	suite.deployment = &models.Deployment{
		AWS: models.NewAWSDeployment(),
	}
	suite.deployment.AWS.RegionalResources = &models.RegionalResources{
		VPCs: make(map[string]*models.AWSVPC),
	}

	// Initialize and configure AWS provider
	var err error
	suite.awsProvider, err = aws_provider.NewAWSProvider("123456789012")
	suite.Require().NoError(err)
	suite.Require().NotNil(suite.awsProvider)
	
	// Set mock clients in provider
	suite.awsProvider.SetSTSClient(suite.mockSTSClient)
	
	// Initialize EC2 clients for each region
	regions := []string{"us-west-2", "us-east-1", "us-west-1"}
	for _, region := range regions {
		mockEC2ClientForRegion := new(aws_mocks.MockEC2Clienter)
		
		// Set up expectations for DescribeAvailabilityZones
		mockEC2ClientForRegion.On("DescribeAvailabilityZones", mock.Anything, mock.AnythingOfType("*ec2.DescribeAvailabilityZonesInput")).
			Return(&ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{
						ZoneName:   aws.String(region + "a"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String(region),
						State:      types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName:   aws.String(region + "b"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String(region),
						State:      types.AvailabilityZoneStateAvailable,
					},
				},
			}, nil)

		// Set up expectations for DescribeInstanceTypes
		mockEC2ClientForRegion.On("DescribeInstanceTypes", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstanceTypesInput")).
			Return(&ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []types.InstanceTypeInfo{
					{
						InstanceType: types.InstanceTypeT2Micro,
					},
					{
						InstanceType: types.InstanceTypeT2Small,
					},
				},
			}, nil)

		suite.awsProvider.SetEC2ClientForRegion(region, mockEC2ClientForRegion)
	}
	suite.awsProvider.SetEC2Client(suite.mockEC2Client)
}
func (suite *CreateDeploymentTestSuite) setupViper() {
	// Create a temporary config file
	tempConfigFile, err := os.CreateTemp("", "aws_test_config_*.yaml")
	suite.Require().NoError(err)
	suite.viperConfigFile = tempConfigFile.Name()
	// Basic AWS configuration
	viper.SetConfigFile(suite.viperConfigFile)
	viper.Set("aws.account_id", "test-account-id")
	viper.Set("aws.default_count_per_zone", 1)
	viper.Set("aws.default_machine_type", "t2.micro")
	viper.Set("aws.default_disk_size_gb", 10)
	viper.Set("aws.regions", []string{"us-west-1"})  // Align with TestPrepareDeployment mock expectations
	viper.Set("general.ssh_private_key_path", suite.testPrivateKeyPath)
	viper.Set("general.ssh_public_key_path", suite.testSSHPublicKeyPath)
	viper.Set("aws.machines", []map[string]interface{}{
		{
			"location": "us-west-1",
			"parameters": map[string]interface{}{
				"count":        1,
				"machine_type": "t2.micro",
				"orchestrator": true,
			},
		},
	})
}
func (suite *CreateDeploymentTestSuite) TestProcessMachinesConfig() {
	tests := []struct {
		name                string
		machinesConfig      []map[string]interface{}
		orchestratorIP      string
		expectError         bool
		expectedErrorString string
		expectedNodes       int
	}{
		{
			name:                "No orchestrator node and no orchestrator IP",
			machinesConfig:      []map[string]interface{}{},
			expectError:         true,
			expectedErrorString: "no orchestrator node and orchestratorIP is not set",
			expectedNodes:       0,
		},
		{
			name: "No orchestrator node but orchestrator IP specified",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-west-1",
					"parameters": map[string]interface{}{"count": 1},
				},
			},
			orchestratorIP: "1.2.3.4",
			expectError:    false,
			expectedNodes:  1,
		},
		{
			name: "One orchestrator node, no other machines",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-west-1",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
			},
			expectError:   false,
			expectedNodes: 1,
		},
		{
			name: "Multiple orchestrator nodes (should error)",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-west-1",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
				{
					"location":   "us-west-2",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
			},
			expectError:         true,
			expectedErrorString: "multiple orchestrator nodes found",
			expectedNodes:       0,
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			deployment, err := models.NewDeployment()
			suite.Require().NoError(err)
			deployment.SSHPrivateKeyPath = suite.testPrivateKeyPath
			deployment.SSHPort = 22
			deployment.OrchestratorIP = tt.orchestratorIP
			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}
			viper.Set("aws.machines", tt.machinesConfig)
			viper.Set("general.orchestrator_ip", tt.orchestratorIP)
			machines, locations, err := suite.awsProvider.ProcessMachinesConfig(suite.ctx)
			if tt.expectError {
				suite.Error(err)
				if tt.expectedErrorString != "" {
					suite.Contains(err.Error(), tt.expectedErrorString)
				}
			} else {
				suite.NoError(err)
				suite.Len(machines, tt.expectedNodes)
				if tt.expectedNodes > 0 {
					suite.NotEmpty(locations)
				}
				if tt.orchestratorIP != "" {
					for _, machine := range machines {
						suite.False(machine.IsOrchestrator())
						suite.Equal(tt.orchestratorIP, machine.GetOrchestratorIP())
					}
				}
			}
		})
	}
}
func (suite *CreateDeploymentTestSuite) TestPrepareDeployment() {
	// Mock EC2 client response for DescribeAvailabilityZones
	suite.mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
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
			},
		}, nil)

	// Create a new deployment with the required fields
	deployment, err := models.NewDeployment()
	suite.Require().NoError(err)
	deployment.DeploymentType = models.DeploymentTypeAWS
	deployment.AWS = &models.AWSDeployment{
		AccountID: "test-account-id",
	}

	deployment.AWS.SetRegionalClient("us-west-1", suite.mockEC2Client)
	deployment.AWS.SetRegionalClient("us-west-2", suite.mockEC2Client)

	// Update the global model with our deployment
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	// Call PrepareDeployment
	err = suite.awsProvider.PrepareDeployment(suite.ctx)
	suite.Require().NoError(err)

	// Get the updated model and verify
	m := display.GetGlobalModelFunc()
	suite.Require().NotNil(m)
	suite.Require().NotNil(m.Deployment)
	suite.Equal(models.DeploymentTypeAWS, m.Deployment.DeploymentType)
	suite.Equal("test-account-id", m.Deployment.AWS.AccountID)
	suite.NotNil(m.Deployment.AWS.RegionalResources)
}

func (suite *CreateDeploymentTestSuite) TestPrepareDeployment_MissingRequiredFields() {
	testCases := []struct {
		name        string
		setupConfig func()
		expectedErr string
	}{
		{
			name: "Missing account_id",
			setupConfig: func() {
				viper.Set("aws.account_id", "")
			},
			expectedErr: "aws.account_id is not set",
		},
		{
			name: "Missing machines configuration",
			setupConfig: func() {
				viper.Set("aws.machines", nil)
			},
			expectedErr: "no machines configuration found",
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupConfig()
			err := suite.awsProvider.PrepareDeployment(suite.ctx)
			suite.Error(err)
			suite.Contains(err.Error(), tc.expectedErr)
		})
	}
}
func (suite *CreateDeploymentTestSuite) TestValidateMachineType() {
	testCases := []struct {
		name        string
		location    string
		machineType string
		expectValid bool
	}{
		{
			name:        "Valid machine type",
			location:    "us-west-1",
			machineType: "t2.micro",
			expectValid: true,
		},
		{
			name:        "Invalid machine type",
			location:    "us-west-1",
			machineType: "invalid.type",
			expectValid: false,
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			isValid, err := suite.awsProvider.ValidateMachineType(
				suite.ctx,
				tc.location,
				tc.machineType,
			)
			if tc.expectValid {
				suite.NoError(err)
				suite.True(isValid)
			} else {
				suite.Error(err)
				suite.False(isValid)
			}
		})
	}
}
func TestCreateDeploymentSuite(t *testing.T) {
	suite.Run(t, new(CreateDeploymentTestSuite))
}

func (s *CreateDeploymentTestSuite) setupTestWithMocks(deploymentType models.DeploymentType) {
	// Reset mock expectations for each test
	s.mockEC2Client.ExpectedCalls = nil
	s.mockSTSClient.ExpectedCalls = nil
	s.mockSSHConfig.ExpectedCalls = nil
	s.mockClusterDeployer.ExpectedCalls = nil

	// Re-establish base STS mock expectations
	s.mockSTSClient.On("GetCallerIdentity", mock.Anything, mock.AnythingOfType("*sts.GetCallerIdentityInput")).
		Return(&sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
			UserId:  aws.String("AIDACKCEVSQ6C2EXAMPLE"),
		}, nil)
		
	// Re-establish EC2 client expectations for each region
	regions := []string{"us-west-2", "us-east-1", "us-west-1"}
	for _, region := range regions {
		mockEC2ClientForRegion := new(aws_mocks.MockEC2Clienter)
		
		// Set up expectations for DescribeAvailabilityZones
		mockEC2ClientForRegion.On("DescribeAvailabilityZones", mock.Anything, mock.AnythingOfType("*ec2.DescribeAvailabilityZonesInput")).
			Return(&ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{
						ZoneName:   aws.String(region + "a"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String(region),
						State:      types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName:   aws.String(region + "b"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String(region),
						State:      types.AvailabilityZoneStateAvailable,
					},
				},
			}, nil)

		// Set up expectations for DescribeInstanceTypes
		mockEC2ClientForRegion.On("DescribeInstanceTypes", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstanceTypesInput")).
			Return(&ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []types.InstanceTypeInfo{
					{
						InstanceType: types.InstanceTypeT2Micro,
					},
					{
						InstanceType: types.InstanceTypeT2Small,
					},
				},
			}, nil)

		s.awsProvider.SetEC2ClientForRegion(region, mockEC2ClientForRegion)
	}

	// Pre-initialize VPCs for all regions we're testing with
	regions := []string{"us-west-2", "us-east-1", "us-west-1"}
	for _, region := range regions {
		// Create a new mock EC2 client for each region
		mockEC2ClientForRegion := new(aws_mocks.MockEC2Clienter)
		
		// Set up expectations for DescribeAvailabilityZones
		mockEC2ClientForRegion.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
			Return(&ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{
						ZoneName:   aws.String(region + "a"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String(region),
						State:      types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName:   aws.String(region + "b"),
						ZoneType:   aws.String("availability-zone"),
						RegionName: aws.String(region),
						State:      types.AvailabilityZoneStateAvailable,
					},
				},
			}, nil).Maybe()

		// Set up spot instance expectations for each region
		mockEC2ClientForRegion.On("RequestSpotInstances", mock.Anything, mock.Anything).
			Return(&ec2.RequestSpotInstancesOutput{
				SpotInstanceRequests: []types.SpotInstanceRequest{
					{
						SpotInstanceRequestId: aws.String(fmt.Sprintf("sir-%s-test123", region)),
						State:                types.SpotInstanceStateActive,
						InstanceId:           aws.String(fmt.Sprintf("i-%s-test123", region)),
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeSpotInstanceRequests", mock.Anything, mock.Anything).
			Return(&ec2.DescribeSpotInstanceRequestsOutput{
				SpotInstanceRequests: []types.SpotInstanceRequest{
					{
						SpotInstanceRequestId: aws.String(fmt.Sprintf("sir-%s-test123", region)),
						State:                types.SpotInstanceStateActive,
						InstanceId:           aws.String(fmt.Sprintf("i-%s-test123", region)),
					},
				},
			}, nil).Maybe()

		// Set up RunInstances expectations for both spot and on-demand instances
		mockEC2ClientForRegion.On("RunInstances", mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
			return input != nil && input.InstanceMarketOptions != nil && 
				input.InstanceMarketOptions.MarketType == types.MarketTypeSpot
		}), mock.Anything).Return(&ec2.RunInstancesOutput{
			Instances: []types.Instance{
				{
					InstanceId:   aws.String(fmt.Sprintf("i-%s-spot123", region)),
					InstanceType: types.InstanceTypeT2Micro,
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
					},
					PublicIpAddress: aws.String("1.2.3.4"),
				},
			},
		}, nil).Maybe()

		mockEC2ClientForRegion.On("RunInstances", mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
			return input != nil && (input.InstanceMarketOptions == nil || 
				input.InstanceMarketOptions.MarketType != types.MarketTypeSpot)
		}), mock.Anything).Return(&ec2.RunInstancesOutput{
			Instances: []types.Instance{
				{
					InstanceId:   aws.String(fmt.Sprintf("i-%s-test123", region)),
					InstanceType: types.InstanceTypeT2Micro,
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
					},
					PublicIpAddress: aws.String("1.2.3.4"),
				},
			},
		}, nil).Maybe()

		// Mock DescribeInstances for both spot and on-demand instances
		mockEC2ClientForRegion.On("DescribeInstances", mock.Anything, mock.Anything).
			Return(&ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:      aws.String(fmt.Sprintf("i-%s-test123", region)),
								InstanceType:    types.InstanceTypeT2Micro,
								PublicIpAddress: aws.String("1.2.3.4"),
								State: &types.InstanceState{
									Name: types.InstanceStateNameRunning,
								},
							},
						},
					},
				},
			}, nil).Maybe()

		// Mock DescribeInstanceStatus
		mockEC2ClientForRegion.On("DescribeInstanceStatus", mock.Anything, mock.Anything).
			Return(&ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []types.InstanceStatus{
					{
						InstanceId: aws.String(fmt.Sprintf("i-%s-test123", region)),
						InstanceState: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						InstanceStatus: &types.InstanceStatusSummary{
							Status: types.SummaryStatusOk,
						},
						SystemStatus: &types.InstanceStatusSummary{
							Status: types.SummaryStatusOk,
						},
					},
				},
			}, nil).Maybe()

		// Mock VPC-related operations
		mockEC2ClientForRegion.On("CreateVpc", mock.Anything, mock.Anything).
			Return(&ec2.CreateVpcOutput{
				Vpc: &types.Vpc{
					VpcId: aws.String(fmt.Sprintf("vpc-%s-test123", region)),
					State: types.VpcStateAvailable,
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("CreateSubnet", mock.Anything, mock.Anything).
			Return(&ec2.CreateSubnetOutput{
				Subnet: &types.Subnet{
					SubnetId: aws.String(fmt.Sprintf("subnet-%s-test123", region)),
					State:    types.SubnetStateAvailable,
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("CreateSecurityGroup", mock.Anything, mock.Anything).
			Return(&ec2.CreateSecurityGroupOutput{
				GroupId: aws.String(fmt.Sprintf("sg-%s-test123", region)),
			}, nil).Maybe()

		mockEC2ClientForRegion.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.Anything).
			Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).Maybe()

		// Mock VPC networking operations
		mockEC2ClientForRegion.On("DescribeVpcs", mock.Anything, mock.Anything).
			Return(&ec2.DescribeVpcsOutput{
				Vpcs: []types.Vpc{
					{
						VpcId: aws.String(fmt.Sprintf("vpc-%s-test123", region)),
						State: types.VpcStateAvailable,
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("CreateInternetGateway", mock.Anything, mock.Anything).
			Return(&ec2.CreateInternetGatewayOutput{
				InternetGateway: &types.InternetGateway{
					InternetGatewayId: aws.String(fmt.Sprintf("igw-%s-test123", region)),
					Attachments: []types.InternetGatewayAttachment{
						{
							State: types.AttachmentStatusAttached,
							VpcId: aws.String(fmt.Sprintf("vpc-%s-test123", region)),
						},
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("AttachInternetGateway", mock.Anything, mock.Anything).
			Return(&ec2.AttachInternetGatewayOutput{}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeInternetGateways", mock.Anything, mock.Anything).
			Return(&ec2.DescribeInternetGatewaysOutput{
				InternetGateways: []types.InternetGateway{
					{
						InternetGatewayId: aws.String(fmt.Sprintf("igw-%s-test123", region)),
						Attachments: []types.InternetGatewayAttachment{
							{
								State: types.AttachmentStatusAttached,
								VpcId: aws.String(fmt.Sprintf("vpc-%s-test123", region)),
							},
						},
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("CreateRouteTable", mock.Anything, mock.Anything).
			Return(&ec2.CreateRouteTableOutput{
				RouteTable: &types.RouteTable{
					RouteTableId: aws.String(fmt.Sprintf("rtb-%s-test123", region)),
					VpcId:       aws.String(fmt.Sprintf("vpc-%s-test123", region)),
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeRouteTables", mock.Anything, mock.Anything).
			Return(&ec2.DescribeRouteTablesOutput{
				RouteTables: []types.RouteTable{
					{
						RouteTableId: aws.String(fmt.Sprintf("rtb-%s-test123", region)),
						VpcId:       aws.String(fmt.Sprintf("vpc-%s-test123", region)),
						Routes: []types.Route{
							{
								DestinationCidrBlock: aws.String("0.0.0.0/0"),
								GatewayId:           aws.String(fmt.Sprintf("igw-%s-test123", region)),
							},
						},
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("CreateRoute", mock.Anything, mock.Anything).
			Return(&ec2.CreateRouteOutput{}, nil).Maybe()

		mockEC2ClientForRegion.On("AssociateRouteTable", mock.Anything, mock.Anything).
			Return(&ec2.AssociateRouteTableOutput{
				AssociationId: aws.String(fmt.Sprintf("rtbassoc-%s-test123", region)),
			}, nil).Maybe()

		// Mock availability zones and subnet operations
		mockEC2ClientForRegion.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
			Return(&ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{
						ZoneName: aws.String(fmt.Sprintf("%sa", region)),
						State:    types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName: aws.String(fmt.Sprintf("%sb", region)),
						State:    types.AvailabilityZoneStateAvailable,
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("CreateSubnet", mock.Anything, mock.MatchedBy(func(input *ec2.CreateSubnetInput) bool {
			return input.VpcId != nil && strings.HasPrefix(*input.VpcId, "vpc-")
		})).Return(&ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{
				SubnetId: aws.String(fmt.Sprintf("subnet-%s-test123", region)),
				VpcId:    aws.String(fmt.Sprintf("vpc-%s-test123", region)),
				State:    types.SubnetStateAvailable,
			},
		}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeSubnets", mock.Anything, mock.Anything).
			Return(&ec2.DescribeSubnetsOutput{
				Subnets: []types.Subnet{
					{
						SubnetId: aws.String(fmt.Sprintf("subnet-%s-test123", region)),
						VpcId:    aws.String(fmt.Sprintf("vpc-%s-test123", region)),
						State:    types.SubnetStateAvailable,
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("ModifySubnetAttribute", mock.Anything, mock.Anything).
			Return(&ec2.ModifySubnetAttributeOutput{}, nil).Maybe()

		// Mock EC2 instance operations
		mockEC2ClientForRegion.On("RunInstances", mock.Anything, mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
			return input.MaxCount != nil && *input.MaxCount == 1
		})).Return(&ec2.RunInstancesOutput{
			Instances: []types.Instance{
				{
					InstanceId:   aws.String(fmt.Sprintf("i-%s-test123", region)),
					InstanceType: types.InstanceTypeT2Micro,
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
						Code: aws.Int32(16),
					},
					PublicIpAddress: aws.String("1.2.3.4"),
				},
			},
		}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeInstances", mock.Anything, mock.Anything).
			Return(&ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:   aws.String(fmt.Sprintf("i-%s-test123", region)),
								InstanceType: types.InstanceTypeT2Micro,
								State: &types.InstanceState{
									Name: types.InstanceStateNameRunning,
									Code: aws.Int32(16),
								},
								PublicIpAddress: aws.String("1.2.3.4"),
							},
						},
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("RequestSpotInstances", mock.Anything, mock.Anything).
			Return(&ec2.RequestSpotInstancesOutput{
				SpotInstanceRequests: []types.SpotInstanceRequest{
					{
						SpotInstanceRequestId: aws.String(fmt.Sprintf("sir-%s-test123", region)),
						State:                 types.SpotInstanceStateOpen,
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeSpotInstanceRequests", mock.Anything, mock.Anything).
			Return(&ec2.DescribeSpotInstanceRequestsOutput{
				SpotInstanceRequests: []types.SpotInstanceRequest{
					{
						SpotInstanceRequestId: aws.String(fmt.Sprintf("sir-%s-test123", region)),
						State:                 types.SpotInstanceStateActive,
						InstanceId:           aws.String(fmt.Sprintf("i-%s-test123", region)),
					},
				},
			}, nil).Maybe()

		mockEC2ClientForRegion.On("WaitUntilInstanceRunning", mock.Anything, mock.Anything).
			Return(nil).Maybe()

		mockEC2ClientForRegion.On("DescribeInstanceStatus", mock.Anything, mock.Anything).
			Return(&ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []types.InstanceStatus{
					{
						InstanceId: aws.String(fmt.Sprintf("i-%s-test123", region)),
						InstanceState: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
							Code: aws.Int32(16),
						},
						InstanceStatus: &types.InstanceStatusSummary{
							Status: types.SummaryStatusOk,
						},
						SystemStatus: &types.InstanceStatusSummary{
							Status: types.SummaryStatusOk,
						},
					},
				},
			}, nil).Maybe()

		// Mock SSH key pair operations
		mockEC2ClientForRegion.On("ImportKeyPair", mock.Anything, mock.Anything).
			Return(&ec2.ImportKeyPairOutput{
				KeyName:        aws.String("andaime-test-key"),
				KeyFingerprint: aws.String("12:34:56:78:90:ab:cd:ef"),
			}, nil).Maybe()

		mockEC2ClientForRegion.On("DescribeKeyPairs", mock.Anything, mock.Anything).
			Return(&ec2.DescribeKeyPairsOutput{
				KeyPairs: []types.KeyPairInfo{
					{
						KeyName:        aws.String("andaime-test-key"),
						KeyFingerprint: aws.String("12:34:56:78:90:ab:cd:ef"),
					},
				},
			}, nil).Maybe()

		// Mock network connectivity check
		mockEC2ClientForRegion.On("DescribeNetworkInterfaces", mock.Anything, mock.Anything).
			Return(&ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(fmt.Sprintf("eni-%s-test123", region)),
						Status:            types.NetworkInterfaceStatusInUse,
						Association: &types.NetworkInterfaceAssociation{
							PublicIp: aws.String("1.2.3.4"),
						},
					},
				},
			}, nil).Maybe()

		// Mock STS client operations
		mockSTSClient := new(aws_mocks.MockSTSClienter)
		mockSTSClient.On("GetCallerIdentity", mock.Anything, mock.Anything).
			Return(&sts.GetCallerIdentityOutput{
				Account: aws.String("123456789012"),
				Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
				UserId:  aws.String("AIDATEST123TEST456TEST"),
			}, nil).Maybe()
		s.awsProvider.SetSTSClient(mockSTSClient)

		// Set the mock EC2 client for each region
		s.deployment.AWS.SetRegionalClient(region, mockEC2ClientForRegion)

		// Pre-initialize VPC for each region
		if _, exists := s.deployment.AWS.RegionalResources.VPCs[region]; !exists {
			s.deployment.AWS.RegionalResources.VPCs[region] = &models.AWSVPC{
				VPCID:           fmt.Sprintf("vpc-%s-initial", region),
				SecurityGroupID: fmt.Sprintf("sg-%s-initial", region),
			}
		}
	}

	// Set up mock responses for VPC operations
	mockEC2Client.On("CreateVpc", mock.Anything, mock.MatchedBy(func(input *ec2.CreateVpcInput) bool {
		return input != nil && input.CidrBlock != nil
	})).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{
				VpcId: aws.String("vpc-test123"),
				State: types.VpcStateAvailable,
			},
		}, nil).
		Maybe()

	// Mock DescribeAvailabilityZones
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
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
					ZoneName:   aws.String("us-west-2a"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-west-2"),
					State:      types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName:   aws.String("us-west-2b"),
					ZoneType:   aws.String("availability-zone"),
					RegionName: aws.String("us-west-2"),
					State:      types.AvailabilityZoneStateAvailable,
				},
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
		}, nil).Maybe()

	// Mock Subnet creation
	mockEC2Client.On("CreateSubnet", mock.Anything, mock.Anything).
		Return(&ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{
				SubnetId: aws.String("subnet-test123"),
			},
		}, nil).Maybe()

	// Mock DescribeVpc
	mockEC2Client.On("DescribeVpcs", mock.Anything, mock.Anything).
		Return(&ec2.DescribeVpcsOutput{
			Vpcs: []types.Vpc{
				{VpcId: aws.String("vpc-test123")},
			},
		}, nil).Maybe()

	// Mock VPC attribute modification
	mockEC2Client.On("ModifyVpcAttribute", mock.Anything, mock.Anything).
		Return(&ec2.ModifyVpcAttributeOutput{}, nil).Maybe()

	// Mock security group creation
	mockEC2Client.On("CreateSecurityGroup", mock.Anything, mock.Anything).
		Return(&ec2.CreateSecurityGroupOutput{
			GroupId: aws.String("sg-test123"),
		}, nil).Maybe()

	// Mock security group rule creation
	mockEC2Client.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.Anything).
		Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).Maybe()

	// Mock VPC cleanup operations
	mockEC2Client.On("DetachInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.DetachInternetGatewayOutput{}, nil).Maybe()
	mockEC2Client.On("DeleteVpc", mock.Anything, mock.Anything).
		Return(&ec2.DeleteVpcOutput{}, nil).Maybe()

	// Mock CreateInternetGateway
	mockEC2Client.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{InternetGatewayId: aws.String("igw-test123")},
		}, nil).Maybe()

	// Mock AttachInternetGateway
	mockEC2Client.On("AttachInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.AttachInternetGatewayOutput{}, nil).Maybe()

	// Mock CreateRouteTable
	mockEC2Client.On("CreateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteTableOutput{
			RouteTable: &types.RouteTable{RouteTableId: aws.String("rtb-test123")},
		}, nil).Maybe()

	// Mock CreateRoute
	mockEC2Client.On("CreateRoute", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteOutput{}, nil).Maybe()

	// Mock AssociateRouteTable
	mockEC2Client.On("AssociateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.AssociateRouteTableOutput{}, nil).Maybe()

	// Mock DescribeInternetGateways
	mockEC2Client.On("DescribeInternetGateways", mock.Anything, mock.Anything).
		Return(&ec2.DescribeInternetGatewaysOutput{
			InternetGateways: []types.InternetGateway{
				{InternetGatewayId: aws.String("igw-test123")},
			},
		}, nil).Maybe()

	// Mock DescribeRouteTables
	mockEC2Client.On("DescribeRouteTables", mock.Anything, mock.Anything).
		Return(&ec2.DescribeRouteTablesOutput{
			RouteTables: []types.RouteTable{
				{
					RouteTableId: aws.String("rtb-test123"),
					Routes: []types.Route{
						{
							GatewayId: aws.String("igw-test123"),
						},
					},
				},
			},
		}, nil).Maybe()
	mockEC2Client.On("DescribeImages", mock.Anything, mock.Anything).
		Return(&ec2.DescribeImagesOutput{
			Images: []types.Image{
				{ImageId: aws.String("ami-test123")},
			},
		}, nil).Maybe()
	// Mock RunInstances for both spot and on-demand instances
	mockEC2Client.On("RunInstances", mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
		// Match spot instance requests
		return input != nil && input.InstanceMarketOptions != nil && 
			input.InstanceMarketOptions.MarketType == types.MarketTypeSpot
	}), mock.Anything).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: aws.String("i-spot123"),
				InstanceType: types.InstanceTypeT2Micro,
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
			},
		},
	}, nil).Maybe()

	// Mock RunInstances for on-demand instances
	mockEC2Client.On("RunInstances", mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
		// Match on-demand instance requests
		return input != nil && (input.InstanceMarketOptions == nil || 
			input.InstanceMarketOptions.MarketType != types.MarketTypeSpot)
	}), mock.Anything).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: aws.String("i-test123"),
				InstanceType: types.InstanceTypeT2Micro,
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
			},
		},
	}, nil).Maybe()

	// Mock DescribeInstances
	// Mock RequestSpotInstances
	mockEC2Client.On("RequestSpotInstances", mock.Anything, mock.Anything).
		Return(&ec2.RequestSpotInstancesOutput{
			SpotInstanceRequests: []types.SpotInstanceRequest{
				{
					SpotInstanceRequestId: aws.String("sir-test123"),
					State:                types.SpotInstanceStateActive,
					InstanceId:           aws.String("i-test123"),
				},
			},
		}, nil).Maybe()

	// Mock DescribeSpotInstanceRequests
	mockEC2Client.On("DescribeSpotInstanceRequests", mock.Anything, mock.Anything).
		Return(&ec2.DescribeSpotInstanceRequestsOutput{
			SpotInstanceRequests: []types.SpotInstanceRequest{
				{
					SpotInstanceRequestId: aws.String("sir-test123"),
					State:                types.SpotInstanceStateActive,
					InstanceId:           aws.String("i-test123"),
				},
			},
		}, nil).Maybe()

	mockEC2Client.On("DescribeInstances", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeInstancesInput) bool {
		// Match any instance ID (both spot and on-demand)
		return input != nil && len(input.InstanceIds) > 0
	}), mock.Anything).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{Instances: []types.Instance{
				{
					InstanceId: aws.String("i-test123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress:  aws.String("1.2.3.4"),
						PrivateIpAddress: aws.String("10.0.0.1"),
					},
				}},
			},
		}, nil).Maybe()

	// Regional clients are already set up with proper expectations in the earlier loop
	// No need to overwrite them with copies of the original mock client

	// Create new mocks for each test
	s.mockSSHConfig = new(sshutils_mocks.MockSSHConfiger)
	s.mockClusterDeployer = new(common_mocks.MockClusterDeployerer)

	// Create mock SSH client
	mockSSHClient := new(sshutils_mocks.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&sshutils_mocks.MockSSHSessioner{}, nil).Maybe()

	// AWS provider and STS client are already set up in SetupTest
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Set up the mock SSH config function
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interfaces.SSHConfiger, error) {
		return s.mockSSHConfig, nil
	}

	// Set up Connect and Close expectations
	s.mockSSHConfig.On("Connect").Return(mockSSHClient, nil).Maybe()
	s.mockSSHConfig.On("Close").Return(nil).Maybe()

	// Set up Connect and Close expectations
	s.mockSSHConfig.On("Connect").Return(mockSSHClient, nil).Maybe()
	s.mockSSHConfig.On("Close").Return(nil).Maybe()

	// Set up default expectations with .Maybe() to make them optional
	s.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()
	s.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()

	// Set up specific Docker command expectation first
	s.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		models.ExpectedDockerHelloWorldCommand,
	).
		Return(models.ExpectedDockerOutput, nil).
		Maybe()

	// Then set up the catch-all for other commands
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(cmd string) bool {
		// Exclude specific commands we want to handle separately
		return cmd != models.ExpectedDockerHelloWorldCommand &&
			!strings.Contains(cmd, "bacalhau node list") &&
			!strings.Contains(cmd, "bacalhau config list")
	})).
		Return("", nil).
		Maybe()

	// Add our specific expectation first
	s.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		models.ExpectedDockerHelloWorldCommand,
	).Return(models.ExpectedDockerOutput, nil).Once()
	s.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 0.0.0.0",
	).Return(`[{"id":"1234567890"}]`, nil).Once()
	s.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"bacalhau node list --output json --api-host 1.2.3.4",
	).Return(`[{"id":"1234567890"}]`, nil).Maybe()
	s.mockSSHConfig.On("ExecuteCommand",
		mock.Anything,
		"sudo bacalhau config list --output json",
	).Return(`[]`, nil).Once()

	s.mockSSHConfig.On("InstallSystemdService",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
	s.mockSSHConfig.On("RestartService",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return("", nil).
		Maybe()
	s.mockSSHConfig.On("InstallBacalhau",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
	s.mockSSHConfig.On("InstallDocker",
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()

	s.mockClusterDeployer.On("ProvisionBacalhauNode",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(nil).
		Maybe()
}

// Add this helper method to verify VPC creation and updates
func (s *CreateDeploymentTestSuite) verifyVPCCreation(region string) {
	m := display.GetGlobalModelFunc()
	vpc, exists := m.Deployment.AWS.RegionalResources.VPCs[region]
	s.Require().True(exists, "VPC should exist for region %s", region)
	s.Require().NotEmpty(vpc.VPCID, "VPC ID should not be empty for region %s", region)
	s.Require().
		NotEmpty(vpc.SecurityGroupID, "Security Group ID should not be empty for region %s", region)
}

// Update the test to verify VPC creation
func (s *CreateDeploymentTestSuite) TestExecuteCreateDeployment() {
	testCases := []struct {
		name           string
		deploymentType models.DeploymentType
		setupConfig    func()
		expectError    bool
	}{
		{
			name:           "spot_instance",
			deploymentType: models.DeploymentTypeAWS,
			setupConfig: func() {
				viper.Set("aws.regions", []string{"us-east-1"})
				viper.Set("aws.spot_instance", true)
				viper.Set("aws.machines", []map[string]interface{}{
					{
						"location": "us-east-1",
						"parameters": map[string]interface{}{
							"count":        1,
							"machine_type": "t2.micro",
							"orchestrator": true,
							"disk_size_gb": 10,
						},
					},
				})
				viper.Set("aws.account_id", "test-account-id")
				viper.Set("aws.default_count_per_zone", 1)
				viper.Set("aws.default_machine_type", "t2.micro")
				viper.Set("aws.default_disk_size_gb", 10)
			},
			expectError: false,
		},
		{
			name:           "on_demand_instance",
			deploymentType: models.DeploymentTypeAWS,
			setupConfig: func() {
				viper.Set("aws.regions", []string{"us-east-1"})
				viper.Set("aws.spot_instance", false)
				viper.Set("aws.machines", []map[string]interface{}{
					{
						"location": "us-east-1",
						"parameters": map[string]interface{}{
							"count":        1,
							"machine_type": "t2.micro",
							"orchestrator": true,
						},
					},
				})
			},
			expectError: false,
		},
		{
			name:           "multiple_regions",
			deploymentType: models.DeploymentTypeAWS,
			setupConfig: func() {
				viper.Set("aws.regions", []string{"us-east-1", "us-west-2"})
				viper.Set("aws.machines", []map[string]interface{}{
					{
						"location": "us-east-1",
						"parameters": map[string]interface{}{
							"count":        1,
							"machine_type": "t2.micro",
							"orchestrator": true,
							"disk_size_gb": 10,
						},
					},
					{
						"location": "us-west-2",
						"parameters": map[string]interface{}{
							"count":        2,
							"machine_type": "t2.micro",
							"disk_size_gb": 10,
						},
					},
				})
				viper.Set("aws.account_id", "test-account-id")
				viper.Set("aws.default_count_per_zone", 1)
				viper.Set("aws.default_machine_type", "t2.micro")
				viper.Set("aws.default_disk_size_gb", 10)
				viper.Set("aws.spot_instance", false)
			},
			expectError: false,
		},
		{
			name:           "invalid_configuration",
			deploymentType: models.DeploymentTypeAWS,
			setupConfig: func() {
				viper.Set("aws.regions", []string{})
				viper.Set("aws.machines", []map[string]interface{}{})
			},
			expectError: true,
		},
	}

	cmd := GetAwsCreateDeploymentCmd()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Reset test state
			s.SetupTest()
			s.setupTestWithMocks(tc.deploymentType)
			tc.setupConfig()

			err := ExecuteCreateDeployment(cmd, []string{})

			if tc.expectError {
				s.Require().Error(err)
				return
			}

			s.Require().NoError(err)

			// Verify VPC creation for each configured region
			regions := viper.GetStringSlice("aws.regions")
			for _, region := range regions {
				s.verifyVPCCreation(region)
			}

			// Verify final deployment state
			s.verifyDeploymentResult(tc.deploymentType)
		})
	}
}

// Add this helper function to separate flag initialization
func addCreateDeploymentFlags(cmd *cobra.Command) {
	cmd.Flags().String("config", "", "Path to the configuration file")
	// Add other necessary flags here
}

func (s *CreateDeploymentTestSuite) verifyDeploymentResult(deploymentType models.DeploymentType) {
	m := display.GetGlobalModelFunc()
	s.Equal(deploymentType, m.Deployment.DeploymentType)
	s.NotNil(m.Deployment.AWS)
	s.NotNil(m.Deployment.AWS.RegionalResources)
	s.NotEmpty(m.Deployment.AWS.RegionalResources.VPCs)

	// Verify each configured machine
	machines := viper.Get("aws.machines").([]map[string]interface{})
	for _, machine := range machines {
		location := machine["location"].(string)
		vpc, exists := m.Deployment.AWS.RegionalResources.VPCs[location]
		s.Require().True(exists)
		s.NotEmpty(vpc.VPCID)
		s.NotEmpty(vpc.SecurityGroupID)
	}
}
