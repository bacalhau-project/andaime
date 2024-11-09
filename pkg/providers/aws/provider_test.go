package awsprovider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cdk_types "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/internal/testutil"
	mocks "github.com/bacalhau-project/andaime/mocks/aws"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const FAKE_ACCOUNT_ID = "123456789012"
const FAKE_REGION = "burkina-faso-1"

func TestNewAWSProvider(t *testing.T) {
	viper.Reset()
	viper.Set("aws.region", FAKE_REGION)
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)

	accountID := viper.GetString("aws.account_id")
	region := viper.GetString("aws.region")
	provider, err := NewAWSProvider(accountID, region)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, region, provider.Region)
}

func TestCreateInfrastructure(t *testing.T) {
	// Store the original function to restore it after the test
	originalNewClient := NewCloudFormationClientFunc
	defer func() { NewCloudFormationClientFunc = originalNewClient }()

	// mockTemplate := `{
	// 	"Resources": {
	// 		"AndaimeVPC": {
	// 			"Type": "AWS::EC2::VPC",
	// 			"Properties": {
	// 				"CidrBlock": "10.0.0.0/16",
	// 				"EnableDnsHostnames": true,
	// 				"EnableDnsSupport": true
	// 			}
	// 		}
	// 	},
	// 	"Outputs": {
	// 		"VpcId": {
	// 			"Value": { "Ref": "AndaimeVPC" }
	// 		}
	// 	}
	// }`

	mockCloudFormationAPI := new(mocks.MockCloudFormationAPIer)

	// Mock CreateStack
	mockCloudFormationAPI.On("CreateStack", mock.Anything, mock.Anything).
		Return(&cloudformation.CreateStackOutput{
			StackId: aws.String("test-stack-id"),
		}, nil)

	// Mock DescribeStacks for the waiter
	mockCloudFormationAPI.On("DescribeStacks", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudformation.DescribeStacksOutput{
			Stacks: []cdk_types.Stack{
				{
					StackName:   aws.String("AndaimeStack"),
					StackId:     aws.String("test-stack-id"),
					StackStatus: cdk_types.StackStatusCreateComplete,
				},
			},
		}, nil)

	// Mock GetTemplate
	// mockCloudFormationAPI.On("GetTemplate", mock.Anything, mock.Anything).
	// 	Return(&cloudformation.GetTemplateOutput{TemplateBody: aws.String(mockTemplate)}, nil)

	NewCloudFormationClientFunc = func(cfg aws.Config) aws_interface.CloudFormationAPIer {
		return mockCloudFormationAPI
	}

	// Test setup
	viper.Reset()
	viper.Set("aws.region", "us-west-2")
	viper.Set("aws.account_id", "123456789012")

	accountID := viper.GetString("aws.account_id")
	region := viper.GetString("aws.region")
	provider, err := NewAWSProvider(accountID, region)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.CreateInfrastructure(ctx)
	assert.NoError(t, err)

	// Verify the infrastructure was created
	assert.NotEmpty(t, provider.VPCID)

	// Verify all mocked calls were made
	mockCloudFormationAPI.AssertExpectations(t)
}

func TestCreateVPC(t *testing.T) {
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	mockEC2Client := new(mocks.MockEC2Clienter)

	// Mock VPC creation
	mockEC2Client.On("CreateVpc", mock.Anything, mock.Anything).
		Return(&ec2.CreateVpcOutput{
			Vpc: &types.Vpc{
				VpcId: aws.String("vpc-12345"),
			},
		}, nil)

	// Mock subnet creation
	mockEC2Client.On("CreateSubnet", mock.Anything, mock.Anything).
		Return(&ec2.CreateSubnetOutput{
			Subnet: &types.Subnet{
				SubnetId: aws.String("subnet-12345"),
			},
		}, nil)

	// Mock internet gateway creation
	mockEC2Client.On("CreateInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.CreateInternetGatewayOutput{
			InternetGateway: &types.InternetGateway{
				InternetGatewayId: aws.String("igw-12345"),
			},
		}, nil)

	// Mock route table creation
	mockEC2Client.On("CreateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteTableOutput{
			RouteTable: &types.RouteTable{
				RouteTableId: aws.String("rtb-12345"),
			},
		}, nil)

	// Mock other necessary EC2 calls
	mockEC2Client.On("AttachInternetGateway", mock.Anything, mock.Anything).
		Return(&ec2.AttachInternetGatewayOutput{}, nil)
	mockEC2Client.On("CreateRoute", mock.Anything, mock.Anything).
		Return(&ec2.CreateRouteOutput{}, nil)
	mockEC2Client.On("AssociateRouteTable", mock.Anything, mock.Anything).
		Return(&ec2.AssociateRouteTableOutput{}, nil)
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
				{
					ZoneName: aws.String("us-west-2a"),
				},
			},
		}, nil)

	provider.SetEC2Client(mockEC2Client)

	err = provider.CreateVPC(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "vpc-12345", provider.VPCID)

	mockEC2Client.AssertExpectations(t)
}

func TestProcessMachinesConfig(t *testing.T) {
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
	require.NoError(t, err)

	ctx := context.Background()
	machines, locations, err := provider.ProcessMachinesConfig(ctx)

	assert.NoError(t, err)
	assert.Len(t, machines, 1)
	assert.Contains(t, locations, "us-west-2")
}

func TestStartResourcePolling(t *testing.T) {
	viper.Reset()
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)
	viper.Set("aws.region", FAKE_REGION)

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = provider.StartResourcePolling(ctx)
	assert.NoError(t, err)
}

func TestValidateMachineType(t *testing.T) {
	viper.Reset()
	viper.Set("aws.region", FAKE_REGION)
	viper.Set("aws.account_id", FAKE_ACCOUNT_ID)

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

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
		t.Run(tc.name, func(t *testing.T) {
			valid, err := provider.ValidateMachineType(ctx, tc.location, tc.instanceType)
			if tc.expectValid {
				assert.NoError(t, err)
				assert.True(t, valid)
			} else {
				assert.Error(t, err)
				assert.False(t, valid)
			}
		})
	}
}

func TestGetVMExternalIP(t *testing.T) {
	// Create a mock EC2 client
	mockEC2Client := &mocks.MockEC2Clienter{}

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
	assert.NoError(t, err)
	assert.Equal(t, "203.0.113.1", ip)

	// Verify that the mock was called as expected
	mockEC2Client.AssertExpectations(t)
}

func TestCreateInfrastructure_Failure(t *testing.T) {
	// Create a mock CloudFormation client
	mockCfnClient := new(mocks.MockCloudFormationAPIer)

	// Set up CreateStack to succeed
	mockCfnClient.On("CreateStack", mock.Anything, mock.AnythingOfType("*cloudformation.CreateStackInput")).
		Return(&cloudformation.CreateStackOutput{}, nil)

	// Set up DescribeStacks to return a failure status
	mockCfnClient.On("DescribeStacks", mock.Anything,
		mock.AnythingOfType("*cloudformation.DescribeStacksInput"),
		mock.Anything).
		Return(&cloudformation.DescribeStacksOutput{
			Stacks: []cdk_types.Stack{
				{
					StackStatus: cdk_types.StackStatusCreateFailed,
				},
			},
		}, nil)

	// Set up DescribeStackEvents for error reporting
	mockCfnClient.On("DescribeStackEvents", mock.Anything, mock.AnythingOfType("*cloudformation.DescribeStackEventsInput")).
		Return(&cloudformation.DescribeStackEventsOutput{
			StackEvents: []cdk_types.StackEvent{
				{
					LogicalResourceId:    aws.String("TestResource"),
					ResourceStatus:       cdk_types.ResourceStatusCreateFailed,
					ResourceStatusReason: aws.String("Test failure reason"),
				},
			},
		}, nil)

	// Create the provider with the mock client
	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID, FAKE_REGION)
	require.NoError(t, err)

	// Override the CloudFormation client creation
	NewCloudFormationClientFunc = func(cfg aws.Config) aws_interface.CloudFormationAPIer {
		return mockCfnClient
	}

	// Call CreateInfrastructure
	err = provider.CreateInfrastructure(context.Background())

	// Assert an error occurred
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "infrastructure stack creation failed")

	// Verify all expected calls were made
	mockCfnClient.AssertExpectations(t)
}
