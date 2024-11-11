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

const FAKE_ACCOUNT_ID = "123456789012"
const FAKE_REGION = "burkina-faso-1"

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

	suite.origLogger = logger.Get() // Save the original logger
	testLogger := logger.NewTestLogger(suite.T())
	logger.SetGlobalLogger(testLogger)
}

func (suite *PkgProvidersAWSProviderSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	sshutils.NewSSHConfigFunc = suite.origNewSSHConfigFunc
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
	accountID := viper.GetString("aws.account_id")
	region := viper.GetString("aws.region")
	provider, err := NewAWSProvider(accountID, region)
	suite.Require().NoError(err)
	suite.Require().NotNil(provider)
	suite.Require().Equal(region, provider.Region)
}

func (suite *PkgProvidersAWSProviderSuite) TestCreateInfrastructure() {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	suite.Require().NoError(err)

	mockEC2Client := new(mocks.MockEC2Clienter)
	// Mock VPC creation
	mockEC2Client.On("CreateVpc", mock.Anything, mock.Anything).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{
				VpcId: aws.String("vpc-12345"),
			},
		}, nil)

	// Mock VPC status check
	mockEC2Client.On("DescribeVpcs", mock.Anything, mock.Anything).
		Return(&ec2.DescribeVpcsOutput{
			Vpcs: []types.Vpc{
				{
					VpcId: aws.String("vpc-12345"),
					State: types.VpcStateAvailable,
				},
			},
		}, nil)
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
				{
					ZoneName: aws.String("FAKE-ZONE"),
				},
			},
		}, nil)
	mockEC2Client.On("CreateSubnet", mock.Anything, mock.Anything).
		Return(&ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{
				SubnetId: aws.String("subnet-12345"),
			},
		}, nil)
	mockEC2Client.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{
				InternetGatewayId: aws.String("igw-12345"),
			},
		}, nil)
	mockEC2Client.On("AttachInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.AttachInternetGatewayOutput{}, nil)
	mockEC2Client.On("CreateRoute", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteOutput{}, nil)
	mockEC2Client.On("AssociateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.AssociateRouteTableOutput{}, nil)
	mockEC2Client.On("CreateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteTableOutput{
			RouteTable: &types.RouteTable{
				RouteTableId: aws.String("rtb-12345"),
			},
		}, nil)

	provider.SetEC2Client(mockEC2Client)

	ctx := context.Background()
	err = provider.CreateInfrastructure(ctx)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(provider.VPCID)
}

func (suite *PkgProvidersAWSProviderSuite) TestCreateVpc() {
	provider, err := NewAWSProviderFunc(FAKE_ACCOUNT_ID, FAKE_REGION)
	suite.Require().NoError(err)

	mockEC2Client := new(mocks.MockEC2Clienter)

	// Mock VPC creation
	mockEC2Client.On("CreateVpc", mock.Anything, mock.Anything).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{
				VpcId: aws.String("vpc-12345"),
			},
		}, nil)

	provider.SetEC2Client(mockEC2Client)

	err = provider.CreateVpc(context.Background())
	suite.Require().NoError(err)
	suite.Require().Equal("vpc-12345", provider.VPCID)
	mockEC2Client.AssertExpectations(suite.T())
}

func (suite *PkgProvidersAWSProviderSuite) TestProcessMachinesConfig() {
	testSSHPublicKeyPath,
		cleanupPublicKey,
		testSSHPrivateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	viper.Reset()
	viper.Set("aws.default_count_per_zone", 1)
	viper.Set("aws.default_machine_type", "t3.micro")
	viper.Set("aws.default_disk_size_gb", 10)
	viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
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

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	suite.Require().NoError(err)

	ctx := context.Background()
	machines, locations, err := provider.ProcessMachinesConfig(ctx)
	suite.Require().NoError(err)
	suite.Require().Len(machines, 1)
	suite.Require().Contains(locations, "us-west-2")
}

func (suite *PkgProvidersAWSProviderSuite) TestStartResourcePolling() {
	viper.Reset()
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)
	viper.Set("aws.region", FAKE_REGION)

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	suite.Require().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = provider.StartResourcePolling(ctx)
	suite.Require().NoError(err)
}

func (suite *PkgProvidersAWSProviderSuite) TestValidateMachineType() {
	viper.Reset()
	viper.Set("aws.region", FAKE_REGION)
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	suite.Require().NoError(err)

	ctx := context.Background()

	testCases := []struct {
		name         string
		location     string
		instanceType string
		expectValid  bool
	}{
		{
			name:         "valid instance type",
			location:     "us-west-2",
			instanceType: "t3.micro",
			expectValid:  true,
		},
		{
			name:         "invalid location",
			location:     "invalid-region",
			instanceType: "t3.micro",
			expectValid:  false,
		},
		{
			name:         "invalid instance type",
			location:     "us-west-2",
			instanceType: "invalid-type",
			expectValid:  false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			valid, err := provider.ValidateMachineType(ctx, tc.location, tc.instanceType)
			if tc.expectValid {
				suite.Require().NoError(err)
				suite.Require().True(valid)
			} else {
				suite.Require().Error(err)
				suite.Require().False(valid)
			}
		})
	}
}

func (suite *PkgProvidersAWSProviderSuite) TestGetVMExternalIP() {
	// Create a mock EC2 client
	mockEC2Client := new(mocks.MockEC2Clienter)

	// Set up the expected call to DescribeInstances
	mockEC2Client.On("DescribeInstances", mock.Anything, &ec2.DescribeInstancesInput{
		InstanceIds: []string{"i-1234567890abcdef0"},
	}).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						PublicIpAddress: aws.String("203.0.113.1"),
					},
				},
			},
		},
	}, nil)

	// Create a provider with the mock EC2 client
	provider := &AWSProvider{
		Region: "us-west-2",
		Config: &aws.Config{},
	}
	provider.SetEC2Client(mockEC2Client)

	// Call the method
	ctx := context.Background()
	ip, err := provider.GetVMExternalIP(ctx, "i-1234567890abcdef0")

	// Assert the results
	suite.Require().NoError(err)
	suite.Require().Equal("203.0.113.1", ip)

	// Verify that the mock was called as expected
	mockEC2Client.AssertExpectations(suite.T())
}

func TestPkgProvidersAWSProviderSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAWSProviderSuite))
}
package awsprovider

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEC2Client struct {
	mock.Mock
}

func (m *mockEC2Client) CreateVpc(ctx context.Context, params *ec2.CreateVpcInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ec2.CreateVpcOutput), args.Error(1)
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func (m *mockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ec2.DescribeVpcsOutput), args.Error(1)
}

func TestResourcePolling(t *testing.T) {
	// Create a mock EC2 client
	mockClient := new(mockEC2Client)

	// Create test provider
	provider := &AWSProvider{
		EC2Client:   mockClient,
		Region:      "us-west-2",
		AccountID:   "123456789012",
		UpdateQueue: make(chan display.UpdateAction, UpdateQueueSize),
	}

	// Create test deployment with machines
	deployment := &models.Deployment{
		Name: "test-deployment",
		Machines: map[string]models.Machiner{
			"test-machine": &models.Machine{
				ID:   "test-machine",
				Name: "test-machine",
			},
		},
	}

	// Set up display model
	model := display.NewDisplayModel(deployment)
	display.SetGlobalModelFunc(func() *display.DisplayModel { return model })

	// Mock EC2 DescribeInstances response
	mockClient.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
		Reservations: []ec2_types.Reservation{
			{
				Instances: []ec2_types.Instance{
					{
						InstanceId: aws.String("i-1234567890"),
						State: &ec2_types.InstanceState{
							Name: ec2_types.InstanceStateNameRunning,
						},
						NetworkInterfaces: []ec2_types.InstanceNetworkInterface{
							{
								Status: aws.String("in-use"),
							},
						},
						BlockDeviceMappings: []ec2_types.InstanceBlockDeviceMapping{
							{
								Ebs: &ec2_types.EbsInstanceBlockDevice{
									Status: aws.String("attached"),
								},
							},
						},
						Tags: []ec2_types.Tag{
							{
								Key:   aws.String("AndaimeMachineID"),
								Value: aws.String("test-machine"),
							},
						},
					},
				},
			},
		},
	}, nil)

	// Test resource polling
	ctx := context.Background()
	err := provider.pollResources(ctx)
	assert.NoError(t, err)

	// Verify machine status updates
	machine := deployment.GetMachine("test-machine")
	assert.Equal(t, models.ResourceStateRunning, machine.GetMachineResourceState(models.AWSResourceTypeInstance.ResourceString))

	// Verify display updates were queued
	select {
	case update := <-provider.UpdateQueue:
		assert.Equal(t, "test-machine", update.MachineName)
		assert.Equal(t, display.UpdateTypeResource, update.UpdateData.UpdateType)
		assert.Equal(t, models.ResourceStateRunning, update.UpdateData.ResourceState)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for update")
	}

	mockClient.AssertExpectations(t)
}

func TestVPCCreation(t *testing.T) {
	// Create a mock EC2 client
	mockClient := new(mockEC2Client)

	// Create test provider
	provider := &AWSProvider{
		EC2Client:   mockClient,
		Region:      "us-west-2",
		AccountID:   "123456789012",
		UpdateQueue: make(chan display.UpdateAction, UpdateQueueSize),
	}

	// Create test deployment with machines
	deployment := &models.Deployment{
		Name: "test-deployment",
		Machines: map[string]models.Machiner{
			"test-machine": &models.Machine{
				ID:   "test-machine",
				Name: "test-machine",
			},
		},
	}

	// Set up display model
	model := display.NewDisplayModel(deployment)
	display.SetGlobalModelFunc(func() *display.DisplayModel { return model })

	// Mock EC2 CreateVpc response
	mockClient.On("CreateVpc", mock.Anything, mock.Anything).Return(&ec2.CreateVpcOutput{
		Vpc: &ec2_types.Vpc{
			VpcId: aws.String("vpc-12345"),
			State: ec2_types.VpcStateAvailable,
		},
	}, nil)

	// Mock EC2 DescribeVpcs response
	mockClient.On("DescribeVpcs", mock.Anything, mock.Anything).Return(&ec2.DescribeVpcsOutput{
		Vpcs: []ec2_types.Vpc{
			{
				VpcId: aws.String("vpc-12345"),
				State: ec2_types.VpcStateAvailable,
			},
		},
	}, nil)

	// Test VPC creation
	ctx := context.Background()
	err := provider.CreateVpc(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "vpc-12345", provider.VPCID)

	// Verify VPC status updates were queued
	select {
	case update := <-provider.UpdateQueue:
		assert.Equal(t, "test-machine", update.MachineName)
		assert.Equal(t, display.UpdateTypeResource, update.UpdateData.UpdateType)
		assert.Equal(t, "VPC", update.UpdateData.ResourceType)
		assert.Equal(t, models.ResourceStateRunning, update.UpdateData.ResourceState)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for update")
	}

	mockClient.AssertExpectations(t)
}
