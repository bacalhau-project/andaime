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
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	EC2Client            aws_interface.EC2Clienter
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
	// Create EC2 client
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)

	// Describe instances
	input := &ec2.DescribeInstancesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("tag:AndaimeDeployment"),
				Values: []string{*p.Stack.StackName()},
			},
		},
	}

	result, err := ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to describe instances: %w", err)
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			// Update instance status in the deployment model
			machineID := getMachineIDFromTags(instance.Tags)
			if machineID != "" {
				status := mapEC2StateToMachineState(instance.State.Name)
				p.updateMachineStatus(machineID, status)
			}
		}
	}

	return nil
}

func getMachineIDFromTags(tags []ec2_types.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "AndaimeMachineID" {
			return *tag.Value
		}
	}
	return ""
}

func mapEC2StateToMachineState(state ec2_types.InstanceStateName) models.MachineResourceState {
	switch state {
	case ec2_types.InstanceStateNamePending:
		return models.ResourceStatePending
	case ec2_types.InstanceStateNameRunning:
		return models.ResourceStateRunning
	case ec2_types.InstanceStateNameStopping:
		return models.ResourceStateStopping
	case ec2_types.InstanceStateNameTerminated:
		return models.ResourceStateTerminated
	default:
		return models.ResourceStateUnknown
	}
}

func (p *AWSProvider) updateMachineStatus(machineID string, status models.MachineResourceState) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		l.Error("Display model or deployment is nil")
		return
	}

	machine := m.Deployment.GetMachine(machineID)
	if machine == nil {
		l.Error(fmt.Sprintf("Machine with ID %s not found", machineID))
		return
	}

	machine.SetMachineResourceState(models.AWSResourceTypeInstance.ResourceString, status)

	// Update the display model
	m.QueueUpdate(display.UpdateAction{
		MachineName: machine.GetName(),
		UpdateData: display.UpdateData{
			UpdateType:    display.UpdateTypeResource,
			ResourceType:  display.ResourceType(models.AWSResourceTypeInstance.ResourceString),
			ResourceState: status,
		},
	})

	l.Debug(fmt.Sprintf("Updated status of machine %s to %d", machineID, status))
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
		Tags: []types.Tag{
			{
				Key:   aws.String("AndaimeDeployment"),
				Value: aws.String("true"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	return nil
}

func (p *AWSProvider) Destroy(ctx context.Context) error {
	l := logger.Get()
	l.Info("Starting destruction of AWS resources")

	// Create CloudFormation client
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	cfnClient := cloudformation.NewFromConfig(cfg)

	// Get the stack name
	stackName := *p.Stack.StackName()

	// Delete the CloudFormation stack
	_, err = cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete CloudFormation stack: %w", err)
	}

	// Wait for the stack to be deleted
	l.Info("Waiting for stack deletion to complete...")
	waiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
	maxWaitTime := 30 * time.Minute
	if err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, maxWaitTime); err != nil {
		return fmt.Errorf("failed waiting for stack deletion: %w", err)
	}

	l.Info("AWS resources successfully destroyed")

	// Clean up local state
	p.Stack = nil
	p.VPC = nil

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
	// Describe the instance
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := p.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe instance: %w", err)
	}

	// Check if we got any reservations and instances
	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("no instance found with ID: %s", instanceID)
	}

	instance := result.Reservations[0].Instances[0]

	// Check if the instance has a public IP address
	if instance.PublicIpAddress == nil {
		return "", fmt.Errorf("instance %s does not have a public IP address", instanceID)
	}

	return *instance.PublicIpAddress, nil
}

func (p *AWSProvider) ListDeployments(ctx context.Context) ([]string, error) {
	l := logger.Get()
	l.Info("Listing AWS deployments")

	// Create CloudFormation client
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	cfnClient := cloudformation.NewFromConfig(cfg)

	// List stacks with our deployment tag
	input := &cloudformation.ListStacksInput{
		StackStatusFilter: []types.StackStatus{
			types.StackStatusCreateComplete,
			types.StackStatusUpdateComplete,
		},
	}

	paginator := cloudformation.NewListStacksPaginator(cfnClient, input)
	var deployments []string

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list stacks: %w", err)
		}

		for _, stack := range output.StackSummaries {
			// Check if the stack has our deployment tag
			describeInput := &cloudformation.DescribeStacksInput{
				StackName: stack.StackName,
			}
			describeOutput, err := cfnClient.DescribeStacks(ctx, describeInput)
			if err != nil {
				l.Warnf("Failed to describe stack %s: %v", *stack.StackName, err)
				continue
			}

			if len(describeOutput.Stacks) > 0 {
				for _, tag := range describeOutput.Stacks[0].Tags {
					if *tag.Key == "AndaimeDeployment" && *tag.Value == "true" {
						deployments = append(deployments, *stack.StackName)
						break
					}
				}
			}
		}
	}

	l.Infof("Found %d AWS deployments", len(deployments))
	return deployments, nil
}

func (p *AWSProvider) TerminateDeployment(ctx context.Context) error {
	l := logger.Get()
	l.Info("Starting termination of AWS deployment")

	if p.Stack == nil {
		return fmt.Errorf("no active stack found for termination")
	}

	// Create CloudFormation client
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	cfnClient := cloudformation.NewFromConfig(cfg)

	// Get the stack name
	stackName := *p.Stack.StackName()

	// Delete the CloudFormation stack
	_, err = cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("failed to initiate stack deletion: %w", err)
	}

	l.Infof("Initiated deletion of stack: %s", stackName)

	// Wait for the stack to be deleted
	l.Info("Waiting for stack deletion to complete...")
	waiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
	maxWaitTime := 30 * time.Minute
	if err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, maxWaitTime); err != nil {
		return fmt.Errorf("error waiting for stack deletion: %w", err)
	}

	l.Info("Stack deletion completed successfully")

	// Clean up local state
	p.Stack = nil
	p.VPC = nil

	// Remove the deployment from the configuration
	if err := p.removeDeploymentFromConfig(stackName); err != nil {
		l.Warnf("Failed to remove deployment from config: %v", err)
	}

	return nil
}

func (p *AWSProvider) removeDeploymentFromConfig(stackName string) error {
	deployments := viper.GetStringMap("deployments")
	for uniqueID, details := range deployments {
		deploymentDetails, ok := details.(map[string]interface{})
		if !ok {
			continue
		}
		awsDetails, ok := deploymentDetails["aws"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, exists := awsDetails[stackName]; exists {
			delete(awsDetails, stackName)
			if len(awsDetails) == 0 {
				delete(deploymentDetails, "aws")
			}
			if len(deploymentDetails) == 0 {
				delete(deployments, uniqueID)
			}
			viper.Set("deployments", deployments)
			return viper.WriteConfig()
		}
	}
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

func (p *AWSProvider) ProvisionBacalhauCluster(ctx context.Context) error {
	if err := p.GetClusterDeployer().ProvisionBacalhauCluster(ctx); err != nil {
		return err
	}

	return nil
}

func (p *AWSProvider) FinalizeDeployment(ctx context.Context) error {
	return nil
}

func (p *AWSProvider) SetEC2Client(client aws_interface.EC2Clienter) {
	p.EC2Client = client
}
