package awsprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	internal_aws "github.com/bacalhau-project/andaime/internal/clouds/aws"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/viper"
)

const (
	ResourcePollingInterval = 10 * time.Second
	UpdateQueueSize         = 100
)

type AWSProvider struct {
	AccountID            string
	Config               *aws.Config
	Region               string
	ClusterDeployer      common_interface.ClusterDeployerer
	UpdateQueue          chan display.UpdateAction
	App                  awscdk.App
	Stack                awscdk.Stack
	VPC                  awsec2.IVpc
	cloudFormationClient aws_interface.CloudFormationAPIer
}

var NewCloudFormationClientFunc = func(cfg aws.Config) aws_interface.CloudFormationAPIer {
	return cloudformation.NewFromConfig(cfg)
}

func NewAWSProvider(accountID string) (*AWSProvider, error) {
	region := viper.GetString("aws.region")

	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	provider := &AWSProvider{
		AccountID:            accountID,
		Config:               &awsConfig,
		Region:               region,
		ClusterDeployer:      common.NewClusterDeployer(models.DeploymentTypeAWS),
		UpdateQueue:          make(chan display.UpdateAction, UpdateQueueSize),
		App:                  awscdk.NewApp(nil),
		cloudFormationClient: NewCloudFormationClientFunc(awsConfig),
	}

	return provider, nil
}

func (p *AWSProvider) PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	return common.PrepareDeployment(ctx, models.DeploymentTypeAWS)
}

// This updates m.Deployment with machines and returns an error if any
func (p *AWSProvider) ProcessMachinesConfig(
	ctx context.Context,
) (map[string]models.Machiner, map[string]bool, error) {
	validateMachineType := func(ctx context.Context, location, machineType string) (bool, error) {
		return p.ValidateMachineType(ctx, location, machineType)
	}

	return common.ProcessMachinesConfig(models.DeploymentTypeAWS, validateMachineType)
}

func (p *AWSProvider) StartResourcePolling(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(ResourcePollingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := p.pollResources(ctx); err != nil {
					logger.Get().Error(fmt.Sprintf("Failed to poll resources: %v", err))
				}
			}
		}
	}()
	return nil
}

func (p *AWSProvider) pollResources(ctx context.Context) error {
	// Implement resource polling using CDK constructs
	return nil
}

func (p *AWSProvider) CreateInfrastructure(ctx context.Context) error {
	cfnClient := NewCloudFormationClientFunc(*p.Config)

	stackProps := &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(p.AccountID),
			Region:  jsii.String(p.Region),
		},
	}

	p.Stack = NewVpcStack(p.App, "AndaimeStack", stackProps)
	p.VPC = p.Stack.Node().FindChild(jsii.String("AndaimeVPC")).(awsec2.IVpc)

	// Get the template
	template := p.App.Synth(nil).GetStackArtifact(jsii.String("AndaimeStack")).Template()

	// Marshal the template to JSON
	templateJSON, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal CloudFormation template: %w", err)
	}

	// Create the stack using the CloudFormation client
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String("AndaimeStack"),
		TemplateBody: aws.String(string(templateJSON)),
	})
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	return nil
}

func (p *AWSProvider) ProvisionBacalhauCluster(ctx context.Context) error {
	// Implement Bacalhau cluster provisioning using CDK constructs
	return nil
}

func (p *AWSProvider) FinalizeDeployment(ctx context.Context) error {
	// Implement any final steps needed for the deployment
	return nil
}

func (p *AWSProvider) Destroy(ctx context.Context) error {
	// Implement resource destruction using CDK constructs
	return nil
}

func (p *AWSProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return p.ClusterDeployer
}

func (p *AWSProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.ClusterDeployer = deployer
}

func NewVpcStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	vpc := awsec2.NewVpc(stack, jsii.String("AndaimeVPC"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				CidrMask:   jsii.Number(24),
				Name:       jsii.String("Public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
			{
				CidrMask:   jsii.Number(24),
				Name:       jsii.String("Private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},
		},
	})

	awscdk.NewCfnOutput(stack, jsii.String("VpcId"), &awscdk.CfnOutputProps{
		Value:       vpc.VpcId(),
		Description: jsii.String("VPC ID"),
	})

	for i, subnet := range *vpc.PublicSubnets() {
		awscdk.NewCfnOutput(
			stack,
			jsii.String(fmt.Sprintf("PublicSubnet%d", i)),
			&awscdk.CfnOutputProps{
				Value:       subnet.SubnetId(),
				Description: jsii.String(fmt.Sprintf("Public Subnet %d ID", i)),
			},
		)
	}

	for i, subnet := range *vpc.PrivateSubnets() {
		awscdk.NewCfnOutput(
			stack,
			jsii.String(fmt.Sprintf("PrivateSubnet%d", i)),
			&awscdk.CfnOutputProps{
				Value:       subnet.SubnetId(),
				Description: jsii.String(fmt.Sprintf("Private Subnet %d ID", i)),
			},
		)
	}

	return stack
}

func (p *AWSProvider) ValidateMachineType(
	ctx context.Context,
	location, instanceType string,
) (bool, error) {
	if internal_aws.IsValidAWSInstanceType(location, instanceType) {
		return true, nil
	}
	if location == "" {
		location = "<NO LOCATION PROVIDED>"
	}

	if instanceType == "" {
		instanceType = "<NO INSTANCE TYPE PROVIDED>"
	}

	return false, fmt.Errorf(
		"invalid instance (%s) and location (%s) for AWS",
		instanceType,
		location,
	)
}

func (p *AWSProvider) GetVMExternalIP(ctx context.Context, instanceID string) (string, error) {
	// Implement getting VM external IP using CDK constructs or AWS SDK
	// This is a placeholder implementation
	return "", fmt.Errorf("not implemented")
}

func (p *AWSProvider) ListDeployments(ctx context.Context) ([]string, error) {
	// Implement listing deployments using CDK constructs or AWS SDK
	// This is a placeholder implementation
	return []string{}, nil
}

func (p *AWSProvider) TerminateDeployment(ctx context.Context) error {
	// Implement terminating deployment using CDK constructs
	// This is a placeholder implementation
	return nil
}

func (p *AWSProvider) ToCloudFormationTemplate(
	ctx context.Context,
	stackName string,
) (map[string]interface{}, error) {
	svc := p.cloudFormationClient

	// Get the template from CloudFormation
	input := &cloudformation.GetTemplateInput{
		StackName: aws.String(stackName),
	}
	result, err := svc.GetTemplate(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("unable to get cloudformation template: %w", err)
	}

	// Deserialize the template body from JSON to map[string]interface{}
	var template map[string]interface{}
	err = json.Unmarshal([]byte(*result.TemplateBody), &template)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal template body: %w", err)
	}

	return template, nil
}
