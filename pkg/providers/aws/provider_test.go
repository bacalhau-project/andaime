package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	aws_mocks "github.com/bacalhau-project/andaime/mocks/aws"
	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	sshutils_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	pkg_testutil "github.com/bacalhau-project/andaime/pkg/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	FAKE_ACCOUNT_ID  = "123456789012"
	FAKE_REGION      = "us-east-1"
	FAKE_VPC_ID      = "vpc-12345"
	FAKE_SUBNET_ID   = "subnet-12345"
	FAKE_IGW_ID      = "igw-12345"
	FAKE_RTB_ID      = "rtb-12345"
	FAKE_INSTANCE_ID = "i-1234567890abcdef0"
)

type PkgProvidersAWSProviderSuite struct {
	suite.Suite
	ctx                    context.Context
	origLogger             *logger.Logger
	testSSHPublicKeyPath   string
	testSSHPrivateKeyPath  string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockAWSClient          *aws_mocks.MockEC2Clienter
	awsProvider            *AWSProvider
	origGetGlobalModelFunc func() *display.DisplayModel
	origNewSSHConfigFunc   func(string, int, string, string) (sshutils_interface.SSHConfiger, error)
	mockSSHConfig          *ssh_mock.MockSSHConfiger
}

func (suite *PkgProvidersAWSProviderSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath,
		suite.cleanupPublicKey,
		suite.testSSHPrivateKeyPath,
		suite.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		deployment, err := models.NewDeployment()
		suite.Require().NoError(err)
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	suite.origLogger = logger.Get()
	logger.SetGlobalLogger(logger.NewTestLogger(suite.T()))
}

func (suite *PkgProvidersAWSProviderSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	sshutils.NewSSHConfigFunc = suite.origNewSSHConfigFunc
	logger.SetGlobalLogger(suite.origLogger)
}

func (suite *PkgProvidersAWSProviderSuite) SetupTest() {
	// Initialize viper configuration
	viper, err := pkg_testutil.InitializeTestViper(testdata.TestAWSConfig)
	require.NoError(suite.T(), err)
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)

	// Create a properly initialized deployment model
	deployment, err := models.NewDeployment()
	require.NoError(suite.T(), err)

	// Initialize AWS deployment structure
	deployment.AWS = &models.AWSDeployment{
		RegionalResources: &models.RegionalResources{
			VPCs:    make(map[string]*models.AWSVPC),
			Clients: make(map[string]aws_interface.EC2Clienter),
		},
	}

	// Set up the global model function
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	// Create a properly initialized AWS provider
	provider, err := NewAWSProviderFunc(FAKE_ACCOUNT_ID)
	require.NoError(suite.T(), err)

	// Set the mock client and ensure it's used for all regions
	suite.mockAWSClient = new(aws_mocks.MockEC2Clienter)
	provider.SetEC2Client(suite.mockAWSClient)

	// Pre-initialize VPC manager with the deployment
	provider.vpcManager = NewRegionalVPCManager(deployment, suite.mockAWSClient)

	// Initialize regional resources for the test region
	deployment.AWS.RegionalResources.VPCs[FAKE_REGION] = &models.AWSVPC{
		VPCID: FAKE_VPC_ID,
	}
	deployment.AWS.RegionalResources.Clients[FAKE_REGION] = suite.mockAWSClient

	// Mock all AWS API calls
	suite.setupAWSMocks()

	// Setup SSH config mock
	suite.mockSSHConfig = new(ssh_mock.MockSSHConfiger)
	suite.origNewSSHConfigFunc = sshutils.NewSSHConfigFunc
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interface.SSHConfiger, error) {
		return suite.mockSSHConfig, nil
	}

	suite.awsProvider = provider
}

func (suite *PkgProvidersAWSProviderSuite) setupAWSMocks() {
	// Mock DescribeRegions
	suite.mockAWSClient.On("DescribeRegions", mock.Anything, mock.Anything).
		Return(&ec2.DescribeRegionsOutput{
			Regions: []types.Region{
				{RegionName: aws.String(FAKE_REGION)},
			},
		}, nil)

	// Mock availability zones
	suite.mockAWSClient.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
				{
					ZoneName:   aws.String(fmt.Sprintf("%sa", FAKE_REGION)),
					State:      types.AvailabilityZoneStateAvailable,
					RegionName: aws.String(FAKE_REGION),
				},
				{
					ZoneName:   aws.String(fmt.Sprintf("%sb", FAKE_REGION)),
					State:      types.AvailabilityZoneStateAvailable,
					RegionName: aws.String(FAKE_REGION),
				},
			},
		}, nil)

	// Mock VPC-related operations
	suite.mockAWSClient.On("CreateVpc", mock.Anything, mock.Anything).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{VpcId: aws.String(FAKE_VPC_ID)},
		}, nil)

	suite.mockAWSClient.On("DescribeVpcs", mock.Anything, mock.Anything).
		Return(&ec2.DescribeVpcsOutput{
			Vpcs: []types.Vpc{
				{
					VpcId: aws.String(FAKE_VPC_ID),
					State: types.VpcStateAvailable,
				},
			},
		}, nil)

	suite.mockAWSClient.On("ModifyVpcAttribute", mock.Anything, mock.Anything).
		Return(&ec2.ModifyVpcAttributeOutput{}, nil)

	// Mock networking components
	suite.mockAWSClient.On("CreateSubnet", mock.Anything, mock.Anything).
		Return(&ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{SubnetId: aws.String(FAKE_SUBNET_ID)},
		}, nil)

	suite.mockAWSClient.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{
				InternetGatewayId: aws.String(FAKE_IGW_ID),
			},
		}, nil)

	suite.mockAWSClient.On("CreateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteTableOutput{
			RouteTable: &types.RouteTable{RouteTableId: aws.String(FAKE_RTB_ID)},
		}, nil)

	// Mock security group operations
	suite.mockAWSClient.On("CreateSecurityGroup", mock.Anything, mock.Anything).
		Return(&ec2.CreateSecurityGroupOutput{
			GroupId: aws.String("sg-12345"),
		}, nil)

	suite.mockAWSClient.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.Anything).
		Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)

	// Mock network setup operations with logging
	suite.mockAWSClient.On("AttachInternetGateway", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(*ec2.AttachInternetGatewayInput)
			logger.Get().Debugf("Attaching Internet Gateway %s to VPC %s",
				*input.InternetGatewayId, *input.VpcId)
		}).
		Return(&ec2.AttachInternetGatewayOutput{}, nil)

	suite.mockAWSClient.On("CreateRoute", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(*ec2.CreateRouteInput)
			logger.Get().Debugf("Creating route in route table %s with destination %s via IGW %s",
				*input.RouteTableId, *input.DestinationCidrBlock, *input.GatewayId)
		}).
		Return(&ec2.CreateRouteOutput{}, nil)

	suite.mockAWSClient.On("AssociateRouteTable", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(*ec2.AssociateRouteTableInput)
			logger.Get().Debugf("Associating route table %s with subnet %s",
				*input.RouteTableId, *input.SubnetId)
		}).
		Return(&ec2.AssociateRouteTableOutput{}, nil)
}

func (suite *PkgProvidersAWSProviderSuite) TestNewAWSProvider() {
	provider, err := NewAWSProviderFunc(FAKE_ACCOUNT_ID)
	suite.Require().NoError(err)
	suite.Require().NotNil(provider)
}

func (suite *PkgProvidersAWSProviderSuite) TestCreateInfrastructure() {
	l := logger.Get()

	// Add debug logging
	l.Info("Starting TestCreateInfrastructure")

	// Check initial state
	m := display.GetGlobalModelFunc()
	if m != nil && m.Deployment != nil {
		l.Debugf("Initial deployment state: %+v", m.Deployment.AWS)
		if m.Deployment.AWS.RegionalResources != nil {
			l.Debugf("VPCs: %+v", m.Deployment.AWS.RegionalResources.VPCs)
		}
	}

	err := suite.awsProvider.CreateInfrastructure(suite.ctx)
	if err != nil {
		l.Debugf("CreateInfrastructure error: %v", err)
		// Print the state after error
		if m != nil && m.Deployment != nil && m.Deployment.AWS != nil {
			l.Debugf("Final deployment state: %+v", m.Deployment.AWS)
			if m.Deployment.AWS.RegionalResources != nil {
				l.Debugf("VPCs: %+v", m.Deployment.AWS.RegionalResources.VPCs)
			}
		}
	}
	suite.Require().NoError(err)
}

func (suite *PkgProvidersAWSProviderSuite) TestCreateVpc() {
	err := suite.awsProvider.CreateVpc(suite.ctx, FAKE_REGION)
	suite.Require().NoError(err)
}

func (suite *PkgProvidersAWSProviderSuite) TestProcessMachinesConfig() {
	viper.Reset()
	viper.Set("aws.default_count_per_zone", 1)
	viper.Set("aws.default_machine_type", "t3.micro")
	viper.Set("aws.default_disk_size_gb", 10)
	viper.Set("general.ssh_private_key_path", suite.testSSHPrivateKeyPath)
	viper.Set("general.ssh_public_key_path", suite.testSSHPublicKeyPath)
	viper.Set("aws.machines", []map[string]interface{}{
		{
			"location": "us-west-2",
			"parameters": map[string]interface{}{
				"count":        1,
				"machine_type": "t3.micro",
				"orchestrator": true,
			},
		},
	})

	machines, locations, err := suite.awsProvider.ProcessMachinesConfig(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().Len(machines, 1)
	suite.Require().Contains(locations, "us-west-2")
}

func (suite *PkgProvidersAWSProviderSuite) TestGetVMExternalIP() {
	suite.mockAWSClient.On("DescribeInstances", mock.Anything, &ec2.DescribeInstancesInput{
		InstanceIds: []string{FAKE_INSTANCE_ID},
	}).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{{
			Instances: []types.Instance{{
				InstanceId:      aws.String(FAKE_INSTANCE_ID),
				PublicIpAddress: aws.String("203.0.113.1"),
			}},
		}},
	}, nil)

	ip, err := suite.awsProvider.GetVMExternalIP(suite.ctx, FAKE_INSTANCE_ID)
	suite.Require().NoError(err)
	suite.Require().Equal("203.0.113.1", ip)
}

func TestPkgProvidersAWSProviderSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAWSProviderSuite))
}
