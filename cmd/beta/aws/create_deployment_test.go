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
	origSSHConfigFunc      sshutils_interfaces.NewSSHConfigFunc
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

	// Set test mode and initialize display model
	os.Setenv("ANDAIME_TEST_MODE", "true")
	model := display.NewDisplayModel(&models.Deployment{})
	display.SetGlobalModel(model)
	display.SetDisplayModelInitialized(true)

	// Verify display model is properly initialized
	suite.Require().NotNil(display.GetGlobalModel(), "Display model must not be nil after initialization")

	// Initialize mock and verification state
	aws.SetAllRegionsMocked(false)
	aws.SetVerificationPassed(false)

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
	
	// All regions have been mocked
	aws.SetAllRegionsMocked(true)
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

	// Set up EC2 client expectations for each region
	testRegions := []string{"us-west-2", "us-east-1", "us-west-1"}
	for _, region := range testRegions {
		mockEC2ClientForRegion := new(aws_mocks.MockEC2Clienter)
		s.setupRegionalMockExpectations(region, mockEC2ClientForRegion)
		s.awsProvider.SetEC2ClientForRegion(region, mockEC2ClientForRegion)
		s.deployment.AWS.SetRegionalClient(region, mockEC2ClientForRegion)

		// Pre-initialize VPC for each region
		if _, exists := s.deployment.AWS.RegionalResources.VPCs[region]; !exists {
			s.deployment.AWS.RegionalResources.VPCs[region] = &models.AWSVPC{
				VPCID:           fmt.Sprintf("vpc-%s-initial", region),
				SecurityGroupID: fmt.Sprintf("sg-%s-initial", region),
			}
		}
	}

	// Set up SSH and cluster deployer mocks
	s.setupSSHMockExpectations()
	s.setupClusterDeployerMockExpectations()
}

func (s *CreateDeploymentTestSuite) setupRegionalMockExpectations(region string, mockEC2ClientForRegion *aws_mocks.MockEC2Clienter) {
	// Set up DescribeAvailabilityZones expectations
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
		}, nil).Maybe()

	// Set up instance-related expectations
	mockEC2ClientForRegion.On("RunInstances", mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
		return input != nil && input.InstanceMarketOptions != nil && 
			input.InstanceMarketOptions.MarketType == types.MarketTypeSpot
	})).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId:   aws.String(fmt.Sprintf("i-%s-spot123", region)),
				InstanceType: types.InstanceTypeT2Micro,
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
				PublicIpAddress:  aws.String("1.2.3.4"),
				PrivateIpAddress: aws.String("10.0.0.1"),
			},
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("RunInstances", mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
		return input != nil && (input.InstanceMarketOptions == nil || 
			input.InstanceMarketOptions.MarketType != types.MarketTypeSpot)
	})).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId:   aws.String(fmt.Sprintf("i-%s-ondemand123", region)),
				InstanceType: types.InstanceTypeT2Micro,
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
				PublicIpAddress:  aws.String("1.2.3.4"),
				PrivateIpAddress: aws.String("10.0.0.1"),
			},
		},
	}, nil).Maybe()

	// Set up VPC and network-related expectations
	mockEC2ClientForRegion.On("CreateVpc", mock.Anything, mock.Anything).Return(&ec2.CreateVpcOutput{
		Vpc: &types.Vpc{
			VpcId: aws.String(fmt.Sprintf("vpc-%s-test", region)),
			State: types.VpcStateAvailable,
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("ModifyVpcAttribute", mock.Anything, mock.Anything).Return(&ec2.ModifyVpcAttributeOutput{}, nil).Maybe()

	mockEC2ClientForRegion.On("DescribeVpcs", mock.Anything, mock.Anything).Return(&ec2.DescribeVpcsOutput{
		Vpcs: []types.Vpc{
			{
				VpcId: aws.String(fmt.Sprintf("vpc-%s-test", region)),
				State: types.VpcStateAvailable,
			},
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("CreateSubnet", mock.Anything, mock.Anything).Return(&ec2.CreateSubnetOutput{
		Subnet: &types.Subnet{
			SubnetId: aws.String(fmt.Sprintf("subnet-%s-test", region)),
			State:    types.SubnetStateAvailable,
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("CreateInternetGateway", mock.Anything, mock.Anything).Return(&ec2.CreateInternetGatewayOutput{
		InternetGateway: &types.InternetGateway{
			InternetGatewayId: aws.String(fmt.Sprintf("igw-%s-test", region)),
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("DescribeInternetGateways", mock.Anything, mock.Anything).Return(&ec2.DescribeInternetGatewaysOutput{
		InternetGateways: []types.InternetGateway{
			{
				InternetGatewayId: aws.String(fmt.Sprintf("igw-%s-test", region)),
			},
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("AttachInternetGateway", mock.Anything, mock.Anything).Return(&ec2.AttachInternetGatewayOutput{}, nil).Maybe()

	mockEC2ClientForRegion.On("CreateRouteTable", mock.Anything, mock.Anything).Return(&ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{
			RouteTableId: aws.String(fmt.Sprintf("rtb-%s-test", region)),
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("DescribeRouteTables", mock.Anything, mock.Anything).Return(&ec2.DescribeRouteTablesOutput{
		RouteTables: []types.RouteTable{
			{
				RouteTableId: aws.String(fmt.Sprintf("rtb-%s-test", region)),
				Routes: []types.Route{
					{
						GatewayId: aws.String(fmt.Sprintf("igw-%s-test", region)),
					},
				},
			},
		},
	}, nil).Maybe()

	mockEC2ClientForRegion.On("CreateRoute", mock.Anything, mock.Anything).Return(&ec2.CreateRouteOutput{}, nil).Maybe()
	mockEC2ClientForRegion.On("AssociateRouteTable", mock.Anything, mock.Anything).Return(&ec2.AssociateRouteTableOutput{}, nil).Maybe()

	// Set up security group expectations
	mockEC2ClientForRegion.On("CreateSecurityGroup", mock.Anything, mock.Anything).Return(&ec2.CreateSecurityGroupOutput{
		GroupId: aws.String(fmt.Sprintf("sg-%s-test", region)),
	}, nil).Maybe()

	mockEC2ClientForRegion.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.Anything).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).Maybe()

	// Mock RunInstances call
	mockEC2ClientForRegion.On("RunInstances", mock.Anything, mock.MatchedBy(func(input *ec2.RunInstancesInput) bool {
		return input != nil &&
			input.ImageId != nil &&
			input.InstanceType == "t2.micro" &&
			len(input.NetworkInterfaces) > 0 &&
			len(input.NetworkInterfaces[0].Groups) > 0 &&
			input.NetworkInterfaces[0].Groups[0] == fmt.Sprintf("sg-%s-test", region)
	})).Return(&ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: aws.String(fmt.Sprintf("i-%s-test", region)),
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
			},
		},
	}, nil)

	// Mock DescribeInstances call
	mockEC2ClientForRegion.On("DescribeInstances", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeInstancesInput) bool {
		return input != nil && len(input.InstanceIds) > 0 && input.InstanceIds[0] == fmt.Sprintf("i-%s-test", region)
	})).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: aws.String(fmt.Sprintf("i-%s-test", region)),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: aws.String("1.2.3.4"),
					},
				},
			},
		},
	}, nil)

	// Mock WaitUntilInstanceRunning call
	mockEC2ClientForRegion.On("WaitUntilInstanceRunning", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeInstancesInput) bool {
		return input != nil && len(input.InstanceIds) > 0 && input.InstanceIds[0] == fmt.Sprintf("i-%s-test", region)
	})).Return(nil)

	// Mock AMI lookup for Ubuntu images with all required filters
	mockEC2ClientForRegion.On("DescribeImages", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeImagesInput) bool {
		if len(input.Filters) != 5 {
			return false
		}
		
		hasNameFilter := false
		hasArchFilter := false
		hasVirtFilter := false
		hasStateFilter := false
		hasOwnerFilter := false

		for _, filter := range input.Filters {
			switch *filter.Name {
			case "name":
				hasNameFilter = len(filter.Values) > 0 && filter.Values[0] == "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-*-server-*"
			case "architecture":
				hasArchFilter = len(filter.Values) > 0 && filter.Values[0] == "x86_64"
			case "virtualization-type":
				hasVirtFilter = len(filter.Values) > 0 && filter.Values[0] == "hvm"
			case "state":
				hasStateFilter = len(filter.Values) > 0 && filter.Values[0] == "available"
			case "owner-id":
				hasOwnerFilter = len(filter.Values) > 0 && filter.Values[0] == "099720109477"
			}
		}

		return hasNameFilter && hasArchFilter && hasVirtFilter && hasStateFilter && hasOwnerFilter
	})).Return(&ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId:      aws.String(fmt.Sprintf("ami-%s-test", region)),
				Name:         aws.String("ubuntu-jammy-22.04-test"),
				Architecture: types.ArchitectureValues("x86_64"),
				VirtualizationType: types.VirtualizationTypeHvm,
				State: types.ImageStateAvailable,
				OwnerId: aws.String("099720109477"),
			},
		},
	}, nil).Maybe()

	// Set up cleanup operation expectations
	mockEC2ClientForRegion.On("DetachInternetGateway", mock.Anything, mock.Anything).Return(&ec2.DetachInternetGatewayOutput{}, nil).Maybe()
	mockEC2ClientForRegion.On("DeleteVpc", mock.Anything, mock.Anything).Return(&ec2.DeleteVpcOutput{}, nil).Maybe()

	// Set up DescribeInstances expectations
	mockEC2ClientForRegion.On("DescribeInstances", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeInstancesInput) bool {
		return input != nil && len(input.InstanceIds) > 0
	}), mock.AnythingOfType("func(*ec2.Options)")).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:   aws.String(fmt.Sprintf("i-%s-test123", region)),
						InstanceType: types.InstanceTypeT2Micro,
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress:  aws.String("1.2.3.4"),
						PrivateIpAddress: aws.String("10.0.0.1"),
					},
				},
			},
		},
	}, nil).Maybe()
}

func (s *CreateDeploymentTestSuite) setupSSHMockExpectations() {
	// Create mock SSH client
	mockSSHClient := new(sshutils_mocks.MockSSHClienter)
	mockSSHClient.On("Close").Return(nil).Maybe()
	mockSSHClient.On("NewSession").Return(&sshutils_mocks.MockSSHSessioner{}, nil).Maybe()
	mockSSHClient.On("GetClient").Return(nil).Maybe()

	// Store original function and set up the mock SSH config function
	s.origSSHConfigFunc = sshutils.NewSSHConfigFunc
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils_interfaces.SSHConfiger, error) {
		return s.mockSSHConfig, nil
	}

	// Set up Connect and Close expectations
	s.mockSSHConfig.On("Connect").Return(mockSSHClient, nil).Maybe()
	s.mockSSHConfig.On("Close").Return(nil).Maybe()

	// Set up default expectations with .Maybe() to make them optional
	s.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// Set up command execution expectations
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, models.ExpectedDockerHelloWorldCommand).
		Return(models.ExpectedDockerOutput, nil).Maybe()

	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(cmd string) bool {
		return cmd != models.ExpectedDockerHelloWorldCommand &&
			!strings.Contains(cmd, "bacalhau node list") &&
			!strings.Contains(cmd, "sudo bacalhau config list")
	})).Return("", nil).Maybe()

	// Set up specific command expectations
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, "bacalhau node list --output json --api-host 0.0.0.0").
		Return(`[{"id":"1234567890"}]`, nil).Once()
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, "bacalhau node list --output json --api-host 1.2.3.4").
		Return(`[{"id":"1234567890"}]`, nil).Maybe()
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo bacalhau config list --output json").
		Return(`[]`, nil).Once()


	// Set up installation expectations
	s.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.mockSSHConfig.On("RestartService", mock.Anything, mock.Anything, mock.Anything).Return("", nil).Maybe()
	s.mockSSHConfig.On("InstallBacalhau", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.mockSSHConfig.On("InstallDocker", mock.Anything, mock.Anything).Return(nil).Maybe()
}

func (s *CreateDeploymentTestSuite) setupClusterDeployerMockExpectations() {
	s.mockClusterDeployer.On("ProvisionBacalhauNode",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
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
