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
	"github.com/bacalhau-project/andaime/internal/testutil"
<<<<<<< HEAD
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
// 	aws_mock "github.com/bacalhau-project/andaime/mocks.*"
// 	sshutils_mock "github.com/bacalhau-project/andaime/mocks.*"
||||||| parent of d27dbd5 (test: add DescribeImages mock for AMI lookup in spot instance tests)
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	sshutils_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
=======
	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	sshutils_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
>>>>>>> d27dbd5 (test: add DescribeImages mock for AMI lookup in spot instance tests)
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	ssh_utils "github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
<<<<<<< HEAD
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

func TestWriteVPCIDToConfig(t *testing.T) {
||||||| parent of d27dbd5 (test: add DescribeImages mock for AMI lookup in spot instance tests)
=======
	"github.com/stretchr/testify/require"
>>>>>>> d27dbd5 (test: add DescribeImages mock for AMI lookup in spot instance tests)
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

type CreateDeploymentTestSuite struct {
	suite.Suite
	ctx                    context.Context
	viperConfigFile        string
	testSSHPublicKeyPath   string
	testPrivateKeyPath     string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockEC2Client          *aws_mock.MockEC2Clienter
	mockSSHConfig          *sshutils_mock.MockSSHConfiger
	awsProvider            *aws_provider.AWSProvider
	origGetGlobalModelFunc func() *display.DisplayModel
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

	suite.mockEC2Client = new(aws_mock.MockEC2Clienter)
	suite.mockSSHConfig = new(sshutils_mock.MockSSHConfiger)
	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		deployment, err := models.NewDeployment()
		suite.Require().NoError(err)
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}
}
func (suite *CreateDeploymentTestSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	if suite.viperConfigFile != "" {
		_ = os.Remove(suite.viperConfigFile)
	}
}
func (suite *CreateDeploymentTestSuite) SetupTest() {
	viper.Reset()
	suite.setupViper()
	suite.mockEC2Client = new(aws_mock.MockEC2Clienter)
	suite.mockSSHConfig = new(sshutils_mock.MockSSHConfiger)
	var err error
	suite.awsProvider, err = aws_provider.NewAWSProvider("test-account-id")
	suite.Require().NoError(err)
	suite.Require().NotNil(suite.awsProvider)
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

	deployment.AWS.RegionalResources = &models.RegionalResources{
		Clients: map[string]aws_interface.EC2Clienter{
			"us-west-1": suite.mockEC2Client,
			"us-west-2": suite.mockEC2Client,
		},
	}

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

func TestExecuteCreateDeployment(t *testing.T) {
	// Create mock EC2 client
	mockEC2Client := new(awsmocks.MockEC2Clienter)

	// Mock DescribeAvailabilityZones response
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, &ec2.DescribeAvailabilityZonesInput{}).
		Return(
			&ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{
						ZoneName:   awsconfig.String("us-east-1a"),
						ZoneType:   awsconfig.String("availability-zone"),
						RegionName: awsconfig.String("us-east-1"),
						State:      types.AvailabilityZoneStateAvailable,
					},
					{
						ZoneName:   awsconfig.String("us-east-1b"),
						ZoneType:   awsconfig.String("availability-zone"),
						RegionName: awsconfig.String("us-east-1"),
						State:      types.AvailabilityZoneStateAvailable,
					},
				},
			}, nil)

	// Mock VPC creation
	mockEC2Client.On("CreateVpc", mock.Anything, mock.MatchedBy(func(input *ec2.CreateVpcInput) bool {
		return input.CidrBlock != nil
	})).Return(&ec2.CreateVpcOutput{
		Vpc: &types.Vpc{
			VpcId: awsconfig.String("vpc-test123"),
			State: types.VpcStateAvailable,
		},
	}, nil)

	// Mock Internet Gateway creation
	mockEC2Client.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{
				InternetGatewayId: awsconfig.String("igw-test123"),
			},
		}, nil)

	// Mock Security Group creation
	mockEC2Client.On("CreateSecurityGroup", mock.Anything, mock.MatchedBy(func(input *ec2.CreateSecurityGroupInput) bool {
		return input.GroupName != nil && input.VpcId != nil
	})).Return(&ec2.CreateSecurityGroupOutput{
		GroupId: awsconfig.String("sg-test123"),
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
			SubnetId: awsconfig.String("subnet-test123"),
			State:    types.SubnetStateAvailable,
		},
	}, nil)

	// Mock route table creation
	mockEC2Client.On("CreateRouteTable", mock.Anything, mock.MatchedBy(func(input *ec2.CreateRouteTableInput) bool {
		return input.VpcId != nil
	})).Return(&ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{
			RouteTableId: awsconfig.String("rtb-test123"),
		},
	}, nil)

	// Mock route creation
	mockEC2Client.On("CreateRoute", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteOutput{}, nil)

	// Mock route table association
	mockEC2Client.On("AssociateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.AssociateRouteTableOutput{
			AssociationId: awsconfig.String("rtbassoc-test123"),
		}, nil)

	// Mock DescribeRouteTables for network connectivity check
	mockEC2Client.On("DescribeRouteTables", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeRouteTablesInput) bool {
		return len(input.Filters) > 0 && input.Filters[0].Values[0] == "vpc-test123"
	})).Return(&ec2.DescribeRouteTablesOutput{
		RouteTables: []types.RouteTable{
			{
				RouteTableId: awsconfig.String("rtb-test123"),
				VpcId:       awsconfig.String("vpc-test123"),
				Routes: []types.Route{
					{
						DestinationCidrBlock: awsconfig.String("0.0.0.0/0"),
						GatewayId:           awsconfig.String("igw-test123"),
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
				VpcId: awsconfig.String("vpc-test123"),
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
				InternetGatewayId: awsconfig.String("igw-test123"),
				Attachments: []types.InternetGatewayAttachment{
					{
						State: types.AttachmentStatusAttached,
						VpcId: awsconfig.String("vpc-test123"),
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
				ImageId: awsconfig.String("ami-test123"),
				Name:    awsconfig.String("amzn2-ami-hvm-2.0.20231218.0-x86_64-gp2"),
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
				InstanceId: awsconfig.String("i-spot123"),
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
				InstanceId: awsconfig.String("i-ondemand123"),
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
			},
		},
	}, nil)

	// Mock DescribeInstances for both spot and on-demand (handles both with and without options)
	mockEC2Client.On(
		"DescribeInstances",
		mock.Anything,  // Use Anything to accept any context implementation
		mock.AnythingOfType("*ec2.DescribeInstancesInput"),
		mock.Anything,  // Use Anything to handle variadic options parameter
	).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: awsconfig.String("i-spot123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: awsconfig.String("1.2.3.4"),
					},
					{
						InstanceId: awsconfig.String("i-ondemand123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: awsconfig.String("5.6.7.8"),
					},
				},
			},
		},
	}, nil)

	// Mock DescribeInstances for direct calls (2-arg version)
	mockEC2Client.On(
		"DescribeInstances",
		mock.Anything,  // Use Anything for context to accept any context implementation
		mock.AnythingOfType("*ec2.DescribeInstancesInput"),
	).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: awsconfig.String("i-spot123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: awsconfig.String("1.2.3.4"),
					},
					{
						InstanceId: awsconfig.String("i-ondemand123"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						PublicIpAddress: awsconfig.String("5.6.7.8"),
					},
				},
			},
		},
	}, nil)
	// Create mock STS client
	mockSTSClient := new(awsmocks.MockSTSClienter)
	mockSTSClient.On("GetCallerIdentity", mock.Anything, mock.Anything).Return(
		&sts.GetCallerIdentityOutput{
			Account: awsconfig.String("123456789012"),
			Arn:     awsconfig.String("arn:aws:iam::123456789012:user/test"),
			UserId:  awsconfig.String("AIDATEST"),
		}, nil)

	// Create SSH client mock
	var mockSSHClient *awsmocks.MockSSHClient
	mockSSHClient = &awsmocks.MockSSHClient{}
	mockSSHClient.ConnectFunc = func() (sshutils.SSHClienter, error) {
		return mockSSHClient, nil
	}
	mockSSHClient.ExecuteCommandFunc = func(ctx context.Context, command string) (string, error) { return "", nil }
	mockSSHClient.IsConnectedFunc = func() bool { return true }
	mockSSHClient.CloseFunc = func() error { return nil }
	mockSSHClient.GetClientFunc = func() *ssh.Client { return nil }
	mockSSHClient.NewSessionFunc = func() (sshutils.SSHSessioner, error) { return nil, nil }

	originalNewAWSProvider := awsprovider.NewAWSProviderFunc
	defer func() {
		awsprovider.NewAWSProviderFunc = originalNewAWSProvider
	}()

	// Mock NewAWSProviderFunc to return our provider with mock clients
	awsprovider.NewAWSProviderFunc = func(accountID string) (*awsprovider.AWSProvider, error) {
		// Create provider with mock clients directly, bypassing AWS credential loading
		cfg := awsconfig.Config{Region: "us-east-1"} // Create config struct
		provider := &awsprovider.AWSProvider{
			AccountID:       accountID,
			Config:         &cfg,
			EC2Client:      mockEC2Client,
			STSClient:      mockSTSClient,
			ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAWS),
			UpdateQueue:     make(chan display.UpdateAction, 1000), // Use constant value directly as it's not exported
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
