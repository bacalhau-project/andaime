package awsprovider

import (
	"context"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/jsii-runtime-go"
	"github.com/bacalhau-project/andaime/internal/testutil"
	mocks "github.com/bacalhau-project/andaime/mocks/aws"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const FAKE_ACCOUNT_ID = "123456789012"

func TestNewAWSProvider(t *testing.T) {
	viper.Reset()
	viper.Set("aws.region", "us-west-2")
	viper.Set("aws.account_id", "123456789012")

	accountID := viper.GetString("aws.account_id")
	provider, err := NewAWSProvider(accountID)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.App)
	assert.Equal(t, "us-west-2", provider.Region)
}

func TestCreateInfrastructure(t *testing.T) {
	// Store the original function to restore it after the test
	originalNewClient := NewCloudFormationClientFunc
	defer func() { NewCloudFormationClientFunc = originalNewClient }()

	mockTemplate := `{
		"Resources": {
			"AndaimeVPC": {
				"Type": "AWS::EC2::VPC",
				"Properties": {
					"CidrBlock": "10.0.0.0/16",
					"EnableDnsHostnames": true,
					"EnableDnsSupport": true
				}
			}
		},
		"Outputs": {
			"VpcId": {
				"Value": { "Ref": "AndaimeVPC" }
			}
		}
	}`

	mockCloudFormationAPI := new(mocks.MockCloudFormationAPIer)
	mockCloudFormationAPI.On("GetTemplate", mock.Anything, mock.Anything).
		Return(&cloudformation.GetTemplateOutput{TemplateBody: aws.String(mockTemplate)}, nil)
	mockCloudFormationAPI.On("CreateStack", mock.Anything, mock.Anything).
		Return(&cloudformation.CreateStackOutput{}, nil)

	NewCloudFormationClientFunc = func(cfg aws.Config) aws_interface.CloudFormationAPIer {
		return mockCloudFormationAPI
	}

	// Test setup
	viper.Reset()
	viper.Set("aws.region", "us-west-2")
	viper.Set("aws.account_id", "123456789012")

	accountID := viper.GetString("aws.account_id")
	provider, err := NewAWSProvider(accountID)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.CreateInfrastructure(ctx)
	assert.NoError(t, err)

	// Verify the infrastructure was created
	assert.NotNil(t, provider.Stack)
	assert.NotNil(t, provider.VPC)

	// Verify VPC properties through the template
	template, err := provider.ToCloudFormationTemplate(ctx, *provider.Stack.StackName())
	require.NoError(t, err)
	resources := template["Resources"].(map[string]interface{})
	assert.Contains(t, resources, "AndaimeVPC")

	outputs := template["Outputs"].(map[string]interface{})
	assert.Contains(t, outputs, "VpcId")
}

func TestNewVpcStack(t *testing.T) {
	ctx := context.Background()

	// Store the original function to restore it after the test
	originalNewClient := NewCloudFormationClientFunc
	defer func() { NewCloudFormationClientFunc = originalNewClient }()

	mockTemplate := &cloudformation.GetTemplateOutput{
		TemplateBody: aws.String(`{
						"Resources": {
							"AndaimeVPC": {
								"Type": "AWS::EC2::VPC",
								"Properties": {
									"CidrBlock": "10.0.0.0/16",
									"EnableDnsHostnames": true,
									"EnableDnsSupport": true
								}
							},
							"AndaimeVPCPublicSubnet1": {
								"Type": "AWS::EC2::Subnet",
								"Properties": {
									"VpcId": { "Ref": "AndaimeVPC" },
									"AvailabilityZone": "us-west-2a",
									"CidrBlock": "10.0.0.0/24",
									"MapPublicIpOnLaunch": true
								}
							},
							"AndaimeVPCPrivateSubnet1": {
								"Type": "AWS::EC2::Subnet",
								"Properties": {
									"VpcId": { "Ref": "AndaimeVPC" },
									"AvailabilityZone": "us-west-2a",
									"CidrBlock": "10.0.1.0/24",
									"MapPublicIpOnLaunch": false
								}
							}
						},
						"Outputs": {
							"VpcId": {
								"Value": { "Ref": "AndaimeVPC" }
							},
							"PublicSubnet1Id": {
								"Value": { "Ref": "AndaimeVPCPublicSubnet1" }
							},
							"PrivateSubnet1Id": {
								"Value": { "Ref": "AndaimeVPCPrivateSubnet1" }
							}
						}
					}`),
	}

	mockCloudFormationAPI := new(mocks.MockCloudFormationAPIer)
	mockCloudFormationAPI.On("GetTemplate", mock.Anything, mock.Anything).
		Return(mockTemplate, nil)
	mockCloudFormationAPI.On("CreateStack", mock.Anything, mock.Anything).
		Return(&cloudformation.CreateStackOutput{}, nil)

	NewCloudFormationClientFunc = func(cfg aws.Config) aws_interface.CloudFormationAPIer {
		return mockCloudFormationAPI
	}

	app := awscdk.NewApp(nil)
	stackProps := &awscdk.StackProps{
		Env: &awscdk.Environment{
			Region: jsii.String("us-west-2"),
		},
	}

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID)
	require.NoError(t, err)

	stack := NewVpcStack(app, "TestStack", stackProps)
	assert.NotNil(t, stack)

	provider.Stack = stack

	// Verify the VPC was created with correct configuration
	template, err := provider.ToCloudFormationTemplate(ctx, *provider.Stack.StackName())
	require.NoError(t, err)
	resources := template["Resources"].(map[string]interface{})

	// Verify VPC configuration
	assert.Contains(t, resources, "AndaimeVPC")
	vpcResource := resources["AndaimeVPC"].(map[string]interface{})
	assert.Equal(t, "AWS::EC2::VPC", vpcResource["Type"])

	// Verify subnet configuration
	assert.Contains(t, resources, "AndaimeVPCPublicSubnet1")
	assert.Contains(t, resources, "AndaimeVPCPrivateSubnet1")

	publicSubnet := resources["AndaimeVPCPublicSubnet1"].(map[string]interface{})
	assert.Equal(t, "AWS::EC2::Subnet", publicSubnet["Type"])
	assert.True(
		t,
		publicSubnet["Properties"].(map[string]interface{})["MapPublicIpOnLaunch"].(bool),
	)

	privateSubnet := resources["AndaimeVPCPrivateSubnet1"].(map[string]interface{})
	assert.Equal(t, "AWS::EC2::Subnet", privateSubnet["Type"])
	assert.False(
		t,
		privateSubnet["Properties"].(map[string]interface{})["MapPublicIpOnLaunch"].(bool),
	)

	// Verify outputs
	outputs := template["Outputs"].(map[string]interface{})
	assert.Contains(t, outputs, "VpcId")
	assert.Contains(t, outputs, "PublicSubnet1Id")
	assert.Contains(t, outputs, "PrivateSubnet1Id")
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

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID)
	require.NoError(t, err)

	ctx := context.Background()
	machines, locations, err := provider.ProcessMachinesConfig(ctx)

	assert.NoError(t, err)
	assert.Len(t, machines, 1)
	assert.Contains(t, locations, "us-west-2")
}

func TestStartResourcePolling(t *testing.T) {
	viper.Reset()
	viper.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = provider.StartResourcePolling(ctx)
	assert.NoError(t, err)
}

func TestValidateMachineType(t *testing.T) {
	viper.Reset()
	viper.Set("aws.region", "us-west-2")

	provider, err := NewAWSProvider(FAKE_ACCOUNT_ID)
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
