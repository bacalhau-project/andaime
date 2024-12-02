package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/gcp"
	"github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"

	"github.com/bacalhau-project/andaime/internal/testdata"

	common_mock "github.com/bacalhau-project/andaime/mocks/common"

	aws_mock "github.com/bacalhau-project/andaime/mocks/aws"
	azure_mock "github.com/bacalhau-project/andaime/mocks/azure"
	gcp_mock "github.com/bacalhau-project/andaime/mocks/gcp"
	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"

	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	azure_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/azure"
	sshutils_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"

	aws_provider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"

	aws_cmd "github.com/bacalhau-project/andaime/cmd/beta/aws"
	azure_cmd "github.com/bacalhau-project/andaime/cmd/beta/azure"
	gcp_cmd "github.com/bacalhau-project/andaime/cmd/beta/gcp"
)

var (
	localScriptPath     = "/tmp/local_custom_script.sh"
	customScriptContent []byte
	customScriptPath    = "/tmp/custom_script.sh"
)

type IntegrationTestSuite struct {
	suite.Suite
	viperConfigFile       string
	cleanup               func()
	testSSHUser           string
	testSSHPublicKeyPath  string
	testSSHPrivateKeyPath string
	mockClusterDeployer   *common_mock.MockClusterDeployerer
	mockSSHConfig         *ssh_mock.MockSSHConfiger
	azureProvider         *azure_provider.AzureProvider
	gcpProvider           *gcp_provider.GCPProvider
	awsProvider           *aws_provider.AWSProvider
}

func (s *IntegrationTestSuite) SetupSuite() {
	os.Setenv("ANDAIME_TEST_MODE", "true")
	publicKeyPath,
		cleanupPublicKey,
		privateKeyPath,
		cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	s.testSSHPublicKeyPath = publicKeyPath
	s.testSSHPrivateKeyPath = privateKeyPath

	f, err := os.CreateTemp("", "local_custom_script.sh")
	s.Require().NoError(err)

	script, err := general.GetLocalCustomScript()
	s.Require().NoError(err)
	customScriptContent = script

	_, err = f.Write(script)
	s.Require().NoError(err)
	localScriptPath = f.Name()

	s.cleanup = func() {
		os.Unsetenv("ANDAIME_TEST_MODE")
		_ = os.Remove(localScriptPath)
		cleanupPublicKey()
		cleanupPrivateKey()
	}

	s.mockClusterDeployer = new(common_mock.MockClusterDeployerer)
	s.mockSSHConfig = new(ssh_mock.MockSSHConfiger)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.cleanup()
}

func (s *IntegrationTestSuite) SetupTest() {
	viper.Reset()
	f, err := os.CreateTemp("", "test-config-*.yaml")
	s.Require().NoError(err)

	var configContent string
	switch viper.GetString("general.deployment_type") {
	case string(models.DeploymentTypeGCP):
		configContent, err = testdata.ReadTestGCPConfig()
	case string(models.DeploymentTypeAWS):
		configContent, err = testdata.ReadTestAWSConfig()
	default:
		configContent, err = testdata.ReadTestAzureConfig()
	}
	s.Require().NoError(err)
	os.WriteFile(f.Name(), []byte(configContent), 0o644)

	s.viperConfigFile = f.Name()
	viper.SetConfigFile(s.viperConfigFile)

	s.setupCommonConfig()
	s.setupMockClusterDeployer()
	s.setupMockSSHConfig()
}

func (s *IntegrationTestSuite) TearDownTest() {
	if s.viperConfigFile != "" {
		_ = os.Remove(s.viperConfigFile)
		s.viperConfigFile = ""
	}
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) setupCommonConfig() {
	viper.Set("general.project_prefix", fmt.Sprintf("test-%d", time.Now().UnixNano()))
	viper.Set("general.ssh_user", "testuser")
	viper.Set("general.ssh_public_key_path", s.testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", s.testSSHPrivateKeyPath)
	viper.Set("general.custom_script_path", localScriptPath)
}

func (s *IntegrationTestSuite) setupProviderConfig(provider models.DeploymentType) {
	switch provider {
	case models.DeploymentTypeAzure:
		viper.Set("azure.subscription_id", "4a45a76b-5754-461d-84a1-f5e47b0a7198")
		viper.Set("azure.default_count_per_zone", 1)
		viper.Set("azure.default_location", "eastus2")
		viper.Set("azure.default_machine_type", "Standard_D4s_v5")
		viper.Set("azure.resource_group_location", "eastus2")
		viper.Set("azure.default_disk_size_gb", 30)
		viper.Set("azure.machines", []interface{}{
			map[string]interface{}{
				"location": "eastus2",
				"parameters": map[string]interface{}{
					"count":        1,
					"orchestrator": true,
				},
			},
			map[string]interface{}{
				"location": "westus",
				"parameters": map[string]interface{}{
					"count": 1,
				},
			},
		})
	case models.DeploymentTypeGCP:
		viper.Set("gcp.project_id", "test-1292-gcp")
		viper.Set("gcp.organization_id", "org-1234567890")
		viper.Set("gcp.billing_account_id", "123456-789012-345678")
		viper.Set("gcp.default_count_per_zone", 1)
		viper.Set("gcp.default_region", "us-central1")
		viper.Set("gcp.default_zone", "us-central1-a")
		viper.Set("gcp.default_machine_type", "n2-standard-2")
		viper.Set("gcp.default_disk_size_gb", 30)
		viper.Set("gcp.machines", []interface{}{
			map[string]interface{}{
				"location": "us-central1-a",
				"parameters": map[string]interface{}{
					"count":        1,
					"orchestrator": true,
				},
			},
			map[string]interface{}{
				"location": "us-central1-b",
				"parameters": map[string]interface{}{
					"count": 1,
				},
			},
		})
	case models.DeploymentTypeAWS:
		viper.Set("aws.account_id", "123456789012")
		viper.Set("aws.default_count_per_zone", 1)
		viper.Set("aws.default_machine_type", "t3.medium")
		viper.Set("aws.default_disk_size_gb", 30)
		viper.Set("aws.region", "us-west-2") // Set default region

		// Configure multi-region deployment with proper orchestrator setup
		viper.Set("aws.machines", []interface{}{
			map[string]interface{}{
				"location": "us-west-2a",
				"parameters": map[string]interface{}{
					"count":        1,
					"orchestrator": true,
				},
			},
			map[string]interface{}{
				"location": "us-east-1a",
				"parameters": map[string]interface{}{
					"count": 1,
				},
			},
		})

		// Configure regional resources
		regions := []string{"us-west-2", "us-east-1"}
		for _, region := range regions {
			viper.Set(fmt.Sprintf("aws.regions.%s.vpc_id", region), "vpc-12345")
			viper.Set(
				fmt.Sprintf("aws.regions.%s.security_group_id", region),
				"sg-1234567890abcdef0",
			)
		}
	}
}

func (s *IntegrationTestSuite) setupDeploymentModel(
	testName string,
	provider models.DeploymentType,
) {
	deploymentName := fmt.Sprintf("test-deployment-%s-%d", testName, time.Now().UnixNano())
	viper.Set("general.project_prefix", deploymentName)

	m := display.GetGlobalModelFunc()
	m.Deployment = &models.Deployment{
		DeploymentType: provider,
		Name:           deploymentName,
	}

	// Initialize provider-specific structures
	switch provider {
	case models.DeploymentTypeAWS:
		m.Deployment.AWS = &models.AWSDeployment{
			AccountID: "123456789012",
			RegionalResources: &models.RegionalResources{
				VPCs:    make(map[string]*models.AWSVPC),
				Clients: make(map[string]aws_interface.EC2Clienter),
			},
		}
		// Initialize VPC maps for each region
		regions := []string{"us-west-2", "us-east-1"}
		for _, region := range regions {
			m.Deployment.AWS.RegionalResources.SetVPC(region, &models.AWSVPC{
				VPCID:           fmt.Sprintf("vpc-%s", region),
				SecurityGroupID: fmt.Sprintf("sg-%s", region),
			})
		}
	case models.DeploymentTypeAzure:
		m.Deployment.Azure = &models.AzureConfig{
			SubscriptionID: "4a45a76b-5754-461d-84a1-f5e47b0a7198",
		}
	case models.DeploymentTypeGCP:
		m.Deployment.GCP = &models.GCPConfig{
			ProjectID:      "test-1292-gcp",
			OrganizationID: "org-1234567890",
		}
	}

	// Set common SSH configuration
	m.Deployment.SSHPublicKeyPath = s.testSSHPublicKeyPath
	m.Deployment.SSHPrivateKeyPath = s.testSSHPrivateKeyPath
}

func (s *IntegrationTestSuite) setupAWSTest() (*cobra.Command, error) {
	cmd := aws_cmd.GetAwsCreateDeploymentCmd()
	cmd.SetContext(context.Background())

	// Initialize deployment model first
	s.setupDeploymentModel("aws-test", models.DeploymentTypeAWS)

	var err error
	s.awsProvider, err = aws_provider.NewAWSProvider("123456789012")
	if err != nil {
		return nil, err
	}

	// Create and configure mock EC2 client
	mockEC2Client := new(aws_mock.MockEC2Clienter)
	s.setupMockAWSResources(mockEC2Client)
	s.awsProvider.SetEC2Client(mockEC2Client)

	// Set up the provider in the command
	aws_provider.NewAWSProviderFunc = func(accountID string) (*aws_provider.AWSProvider, error) {
		return s.awsProvider, nil
	}

	// Initialize the deployment model with AWS-specific configuration
	m := display.GetGlobalModelFunc()
	if m.Deployment != nil && m.Deployment.AWS != nil {
		// Initialize VPCs map with required regions
		m.Deployment.AWS.RegionalResources.VPCs = map[string]*models.AWSVPC{
			"us-west-2": {
				VPCID:           "vpc-12345",
				SecurityGroupID: "sg-1234567890abcdef0",
			},
			"us-east-1": {
				VPCID:           "vpc-12345",
				SecurityGroupID: "sg-1234567890abcdef0",
			},
		}
		// Set up EC2 clients for each region
		for region := range m.Deployment.AWS.RegionalResources.VPCs {
			m.Deployment.AWS.RegionalResources.Clients[region] = mockEC2Client
		}
	}

	return cmd, nil
}

func (s *IntegrationTestSuite) setupMockAWSResources(mockEC2Client *aws_mock.MockEC2Clienter) {
	// Region mocks
	mockEC2Client.On("DescribeRegions", mock.Anything, mock.AnythingOfType("*ec2.DescribeRegionsInput")).
		Return(&ec2.DescribeRegionsOutput{
			Regions: []types.Region{
				{
					RegionName: aws.String("us-west-2"),
					Endpoint:   aws.String("ec2.us-west-2.amazonaws.com"),
				},
				{
					RegionName: aws.String("us-east-1"),
					Endpoint:   aws.String("ec2.us-east-1.amazonaws.com"),
				},
			},
		}, nil)
	mockEC2Client.On("DescribeAvailabilityZones", mock.Anything, mock.AnythingOfType("*ec2.DescribeAvailabilityZonesInput")).
		Return(&ec2.DescribeAvailabilityZonesOutput{
			AvailabilityZones: []types.AvailabilityZone{
				{
					ZoneName: aws.String("us-west-2a"),
					State:    types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName: aws.String("us-west-2b"),
					State:    types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName: aws.String("us-east-1a"),
					State:    types.AvailabilityZoneStateAvailable,
				},
				{
					ZoneName: aws.String("us-east-1b"),
					State:    types.AvailabilityZoneStateAvailable,
				},
			},
		}, nil)

	// VPC and Networking mocks
	mockEC2Client.On("CreateVpc", mock.Anything, mock.AnythingOfType("*ec2.CreateVpcInput")).
		Return(testdata.FakeEC2CreateVpcOutput(), nil)
	mockEC2Client.On("DescribeVpcs", mock.Anything, mock.AnythingOfType("*ec2.DescribeVpcsInput")).
		Return(testdata.FakeEC2DescribeVpcsOutput(), nil)
	mockEC2Client.On("ModifyVpcAttribute", mock.Anything, mock.AnythingOfType("*ec2.ModifyVpcAttributeInput")).
		Return(testdata.FakeEC2ModifyVpcAttributeOutput(), nil)
	mockEC2Client.On("CreateSecurityGroup", mock.Anything, mock.AnythingOfType("*ec2.CreateSecurityGroupInput")).
		Return(testdata.FakeEC2CreateSecurityGroupOutput(), nil)
	mockEC2Client.On("AuthorizeSecurityGroupIngress", mock.Anything, mock.AnythingOfType("*ec2.AuthorizeSecurityGroupIngressInput")).
		Return(testdata.FakeAuthorizeSecurityGroupIngressOutput(), nil)

	// Routing mocks
	mockEC2Client.On("CreateInternetGateway", mock.Anything, mock.AnythingOfType("*ec2.CreateInternetGatewayInput")).
		Return(testdata.FakeEC2CreateInternetGatewayOutput(), nil)
	mockEC2Client.On("AttachInternetGateway", mock.Anything, mock.AnythingOfType("*ec2.AttachInternetGatewayInput")).
		Return(testdata.FakeEC2AttachInternetGatewayOutput(), nil)
	mockEC2Client.On("CreateRouteTable", mock.Anything, mock.AnythingOfType("*ec2.CreateRouteTableInput")).
		Return(testdata.FakeEC2CreateRouteTableOutput(), nil)
	mockEC2Client.On("CreateRoute", mock.Anything, mock.AnythingOfType("*ec2.CreateRouteInput")).
		Return(testdata.FakeEC2CreateRouteOutput(), nil)
	mockEC2Client.On("AssociateRouteTable", mock.Anything, mock.AnythingOfType("*ec2.AssociateRouteTableInput")).
		Return(testdata.FakeEC2AssociateRouteTableOutput(), nil)
	mockEC2Client.On("DescribeRouteTables", mock.Anything, mock.AnythingOfType("*ec2.DescribeRouteTablesInput")).
		Return(testdata.FakeEC2DescribeRouteTablesOutput(), nil)

	// Instance mocks
	mockEC2Client.On("RunInstances", mock.Anything, mock.AnythingOfType("*ec2.RunInstancesInput")).
		Return(testdata.FakeEC2RunInstancesOutput(), nil)
	mockEC2Client.On("DescribeInstances", mock.Anything,
		mock.AnythingOfType("*ec2.DescribeInstancesInput"),
		mock.Anything).
		Return(testdata.FakeEC2DescribeInstancesOutput(), nil)
	mockEC2Client.On("DescribeImages", mock.Anything, mock.AnythingOfType("*ec2.DescribeImagesInput")).
		Return(testdata.FakeEC2DescribeImagesOutput(), nil)
	mockEC2Client.On("CreateTags", mock.Anything, mock.AnythingOfType("*ec2.CreateTagsInput")).
		Return(&ec2.CreateTagsOutput{}, nil)

	// Subnet mocks
	mockEC2Client.On("CreateSubnet", mock.Anything, mock.AnythingOfType("*ec2.CreateSubnetInput")).
		Return(testdata.FakeEC2CreateSubnetOutput(), nil)
	mockEC2Client.On("DescribeSubnets", mock.Anything, mock.AnythingOfType("*ec2.DescribeSubnetsInput")).
		Return(testdata.FakeEC2DescribeSubnetsOutput(), nil)
}

func (s *IntegrationTestSuite) setupMockClusterDeployer() {
	s.mockClusterDeployer.On("CreateResources", mock.Anything).Return(nil)
	s.mockClusterDeployer.On("ProvisionMachines", mock.Anything).Return(nil)
	s.mockClusterDeployer.On("ProvisionBacalhauCluster", mock.Anything).Return(nil)
	s.mockClusterDeployer.On("FinalizeDeployment", mock.Anything).Return(nil)
	s.mockClusterDeployer.On("ProvisionOrchestrator", mock.Anything, mock.Anything).Return(nil)
	s.mockClusterDeployer.On("ProvisionWorker", mock.Anything, mock.Anything).Return(nil)
	s.mockClusterDeployer.On("ProvisionPackagesOnMachine", mock.Anything, mock.Anything).Return(nil)
	s.mockClusterDeployer.On("WaitForAllMachinesToReachState", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	s.mockClusterDeployer.On("ProvisionBacalhau", mock.Anything).Return(nil)
	s.mockClusterDeployer.On("PrepareDeployment", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, deploymentType models.DeploymentType) (*models.Deployment, error) {
			return &models.Deployment{DeploymentType: deploymentType}, nil
		})
	s.mockClusterDeployer.On("GetConfig").Return(viper.New())
	s.mockClusterDeployer.On("SetConfig", mock.Anything).Return(nil)
}

func (s *IntegrationTestSuite) setupMockSSHConfig() {
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker run hello-world").
		Return("Hello from Docker!", nil)
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).
		Return(`[{"id": "node1", "public_ip": "1.2.3.4"}]`, nil)
	s.mockSSHConfig.On("ExecuteCommand", mock.Anything, fmt.Sprintf("sudo bash %s", customScriptPath)).
		Return("", nil)
	s.mockSSHConfig.On("PushFile", mock.Anything, customScriptPath, customScriptContent, mock.Anything).
		Return(nil)
	s.mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	s.mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	s.mockSSHConfig.On("RestartService", mock.Anything, mock.Anything).Return("", nil)
	s.mockSSHConfig.On("WaitForSSH",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interface.SSHConfiger, error) {
		return s.mockSSHConfig, nil
	}
}

func (s *IntegrationTestSuite) TestExecuteCreateDeployment() {
	tests := []struct {
		name     string
		provider models.DeploymentType
		setup    func() (*cobra.Command, error)
	}{
		{
			name:     "aws",
			provider: models.DeploymentTypeAWS,
			setup:    s.setupAWSTest,
		},
		{
			name:     "azure",
			provider: models.DeploymentTypeAzure,
			setup:    s.setupAzureTest,
		},
		{
			name:     "gcp",
			provider: models.DeploymentTypeGCP,
			setup:    s.setupGCPTest,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.setupProviderConfig(tt.provider)
			s.setupDeploymentModel(tt.name, tt.provider)

			cmd, err := tt.setup()
			s.Require().NoError(err)

			err = s.executeDeployment(cmd, tt.provider)
			s.Require().NoError(err)

			s.verifyDeploymentResult(tt.provider)
		})
	}
}

func (s *IntegrationTestSuite) setupAzureTest() (*cobra.Command, error) {
	cmd := azure.GetAzureCreateDeploymentCmd()
	cmd.SetContext(context.Background())

	var err error
	s.azureProvider, err = azure_provider.NewAzureProvider(
		context.Background(),
		viper.GetString("azure.subscription_id"),
	)
	if err != nil {
		return nil, err
	}

	mockPoller := new(MockPoller)
	mockAzureClient := new(azure_mock.MockAzureClienter)
	s.setupAzureMocks(mockAzureClient, mockPoller)

	azure_provider.NewAzureProviderFunc = func(ctx context.Context, subscriptionID string) (*azure_provider.AzureProvider, error) {
		return s.azureProvider, nil
	}

	azure_provider.NewAzureClientFunc = func(subscriptionID string) (azure_interface.AzureClienter, error) {
		return mockAzureClient, nil
	}

	return cmd, nil
}

func (s *IntegrationTestSuite) setupAzureMocks(
	mockAzureClient *azure_mock.MockAzureClienter,
	mockPoller *MockPoller,
) {
	mockPoller.On("PollUntilDone", mock.Anything, mock.Anything).
		Return(armresources.DeploymentsClientCreateOrUpdateResponse{
			DeploymentExtended: testdata.FakeDeployment(),
		}, nil)

	mockAzureClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)
	mockAzureClient.On("GetOrCreateResourceGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeResourceGroup(), nil)
	mockAzureClient.On("DeployTemplate",
		mock.Anything,
		mock.Anything,
		mock.MatchedBy(func(s string) bool {
			return strings.HasPrefix(s, "deployment-") ||
				strings.HasPrefix(s, "machine-")
		}),
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mockPoller, nil)
	mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeVirtualMachine(), nil)
	mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeNetworkInterface(), nil)
	mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)
}

func (s *IntegrationTestSuite) setupGCPTest() (*cobra.Command, error) {
	cmd := gcp.GetGCPCreateDeploymentCmd()
	cmd.SetContext(context.Background())

	mockGCPClient := new(gcp_mock.MockGCPClienter)
	s.setupGCPMocks(mockGCPClient)

	var err error
	s.gcpProvider, err = gcp_provider.NewGCPProvider(
		context.Background(),
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	if err != nil {
		return nil, err
	}

	s.gcpProvider.SetGCPClient(mockGCPClient)

	gcp_provider.NewGCPProviderFunc = func(ctx context.Context, organizationID string, billingAccountID string) (*gcp_provider.GCPProvider, error) {
		return s.gcpProvider, nil
	}

	return cmd, nil
}

func (s *IntegrationTestSuite) setupGCPMocks(mockGCPClient *gcp_mock.MockGCPClienter) {
	mockGCPClient.On("IsAPIEnabled", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)
	mockGCPClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)
	mockGCPClient.On("EnsureProject", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("test-1292-gcp", nil)
	mockGCPClient.On("EnableAPI", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	mockGCPClient.On("CreateVPCNetwork", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	mockGCPClient.On("ListAddresses", mock.Anything, mock.Anything, mock.Anything).
		Return([]*computepb.Address{testdata.FakeGCPIPAddress()}, nil)
	mockGCPClient.On("CreateIP", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	mockGCPClient.On("CreateFirewallRules", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	mockGCPClient.On("CreateVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPInstance(), nil)
}

func (s *IntegrationTestSuite) executeDeployment(
	cmd *cobra.Command,
	provider models.DeploymentType,
) error {
	switch provider {
	case models.DeploymentTypeAWS:
		return aws_cmd.ExecuteCreateDeployment(cmd, []string{})
	case models.DeploymentTypeAzure:
		return azure_cmd.ExecuteCreateDeployment(cmd, []string{})
	case models.DeploymentTypeGCP:
		return gcp_cmd.ExecuteCreateDeployment(cmd, []string{})
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (s *IntegrationTestSuite) verifyDeploymentResult(provider models.DeploymentType) {
	m := display.GetGlobalModelFunc()
	s.NotEmpty(m.Deployment.Name)
	s.Equal(
		strings.ToLower(string(provider)),
		strings.ToLower(string(m.Deployment.DeploymentType)),
	)
	s.NotEmpty(m.Deployment.Machines)
	s.Equal(s.testSSHPublicKeyPath, m.Deployment.SSHPublicKeyPath)
	s.Equal(s.testSSHPrivateKeyPath, m.Deployment.SSHPrivateKeyPath)

	switch provider {
	case models.DeploymentTypeAzure:
		s.Equal("4a45a76b-5754-461d-84a1-f5e47b0a7198", m.Deployment.Azure.SubscriptionID)
	case models.DeploymentTypeGCP:
		s.Equal("test-1292-gcp", m.Deployment.GCP.ProjectID)
		s.Equal("org-1234567890", m.Deployment.GCP.OrganizationID)
	case models.DeploymentTypeAWS:
		// Verify that VPCs exist in the configured regions
		s.NotEmpty(m.Deployment.AWS.RegionalResources.VPCs)
		for region, vpc := range m.Deployment.AWS.RegionalResources.VPCs {
			s.NotEmpty(
				vpc.VPCID,
				"VPC ID should not be empty for region %s",
				region,
			)
			s.NotEmpty(
				vpc.SecurityGroupID,
				"Security Group ID should not be empty for region %s",
				region,
			)
		}
	}
}

// MockPoller implementation for Azure deployment polling
type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) PollUntilDone(
	ctx context.Context,
	options *runtime.PollUntilDoneOptions,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) ResumeToken() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockPoller) Result(
	ctx context.Context,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) Done() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockPoller) Poll(ctx context.Context) (*http.Response, error) {
	args := m.Called(ctx)
	return args.Get(0).(*http.Response), args.Error(1)
}
