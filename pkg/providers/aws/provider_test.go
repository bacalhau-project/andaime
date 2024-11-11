package awsprovider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	mocks "github.com/bacalhau-project/andaime/mocks/aws"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	pkg_testutil "github.com/bacalhau-project/andaime/pkg/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	FAKE_ACCOUNT_ID  = "123456789012"
	FAKE_REGION      = "burkina-faso-1"
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
	mockAWSClient          *mocks.MockEC2Clienter
	awsProvider            *AWSProvider
	origGetGlobalModelFunc func() *display.DisplayModel
	origNewSSHConfigFunc   func(string, int, string, string) (sshutils.SSHConfiger, error)
	mockSSHConfig          *sshutils.MockSSHConfig
}

func (suite *PkgProvidersAWSProviderSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath,
		suite.cleanupPublicKey,
		suite.testSSHPrivateKeyPath,
		suite.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	suite.mockAWSClient = new(mocks.MockEC2Clienter)
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
	viper, err := pkg_testutil.InitializeTestViper(testdata.TestAWSConfig)
	require.NoError(suite.T(), err)
	viper.Set("aws.region", FAKE_REGION)
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)

	suite.awsProvider = &AWSProvider{}
	suite.awsProvider.SetEC2Client(suite.mockAWSClient)

	suite.mockSSHConfig = new(sshutils.MockSSHConfig)
	suite.origNewSSHConfigFunc = sshutils.NewSSHConfigFunc
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return suite.mockSSHConfig, nil
	}
}

func (suite *PkgProvidersAWSProviderSuite) TestNewAWSProvider() {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	suite.Require().NoError(err)
	suite.Require().NotNil(provider)
	suite.Require().Equal(FAKE_REGION, provider.Region)
}

func (suite *PkgProvidersAWSProviderSuite) TestCreateInfrastructure() {
	// Mock VPC creation
	suite.mockAWSClient.On("CreateVpc", mock.Anything, mock.Anything).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{VpcId: aws.String(FAKE_VPC_ID)},
		}, nil)

	// Mock VPC status check
	suite.mockAWSClient.On("DescribeVpcs", mock.Anything, mock.Anything).
		Return(&ec2.DescribeVpcsOutput{
			Vpcs: []types.Vpc{{
				VpcId: aws.String(FAKE_VPC_ID),
				State: types.VpcStateAvailable,
			}},
		}, nil)

	// Mock availability zones
	suite.mockAWSClient.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{{
				ZoneName: aws.String("FAKE-ZONE"),
			}},
		}, nil)

	// Mock subnet creation
	suite.mockAWSClient.On("CreateSubnet", mock.Anything, mock.Anything).
		Return(&ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{SubnetId: aws.String(FAKE_SUBNET_ID)},
		}, nil)

	// Mock internet gateway creation
	suite.mockAWSClient.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{
				InternetGatewayId: aws.String(FAKE_IGW_ID),
			},
		}, nil)

	// Mock route table creation
	suite.mockAWSClient.On("CreateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteTableOutput{
			RouteTable: &types.RouteTable{RouteTableId: aws.String(FAKE_RTB_ID)},
		}, nil)

	// Mock remaining network setup with detailed logging
	suite.mockAWSClient.On("AttachInternetGateway", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(*ec2.AttachInternetGatewayInput)
			logger.Get().Debugf("Attaching Internet Gateway %s to VPC %s", *input.InternetGatewayId, *input.VpcId)
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
			logger.Get().Debugf("Associating route table %s with subnet %s", *input.RouteTableId, *input.SubnetId)
		}).
		Return(&ec2.AssociateRouteTableOutput{}, nil)

	err := suite.awsProvider.CreateInfrastructure(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(FAKE_VPC_ID, suite.awsProvider.VPCID)
}

func (suite *PkgProvidersAWSProviderSuite) TestCreateVpc() {
	suite.mockAWSClient.On("CreateVpc", mock.Anything, mock.Anything).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{VpcId: aws.String(FAKE_VPC_ID)},
		}, nil)

	err := suite.awsProvider.CreateVpc(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(FAKE_VPC_ID, suite.awsProvider.VPCID)
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

func (suite *PkgProvidersAWSProviderSuite) TestGetLatestUbuntuAMI() {
	creationDate := time.Now().Format(time.RFC3339)
	suite.mockAWSClient.On("DescribeImages", mock.Anything, mock.Anything).
		Return(&ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      aws.String("ami-12345"),
					CreationDate: aws.String(creationDate),
					State:        types.ImageStateAvailable,
				},
			},
		}, nil)

	amiID, err := suite.awsProvider.GetLatestUbuntuAMI(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().Equal("ami-12345", amiID)
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
