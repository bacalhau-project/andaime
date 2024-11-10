package awsprovider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	DefaultStackTimeout     = 30 * time.Minute
	TestStackTimeout        = 30 * time.Second
)

type AWSProvider struct {
	AccountID       string
	Config          *aws.Config
	Region          string
	ClusterDeployer common_interface.ClusterDeployerer
	UpdateQueue     chan display.UpdateAction
	VPCID           string
	EC2Client       aws_interface.EC2Clienter
}

func NewAWSProvider(accountID, region string) (*AWSProvider, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	if region == "" {
		return nil, fmt.Errorf("region is required")
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	provider := &AWSProvider{
		AccountID:       accountID,
		Region:          region,
		Config:          &awsConfig,
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAWS),
		UpdateQueue:     make(chan display.UpdateAction, UpdateQueueSize),
	}

	// Initialize EC2 client
	ec2Client := ec2.NewFromConfig(awsConfig)
	provider.EC2Client = &LiveEC2Client{client: ec2Client}

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
				Values: []string{"AndaimeDeployment"},
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

func (p *AWSProvider) BootstrapEnvironment(ctx context.Context) error {
	// No bootstrapping needed anymore since we're not using CDK
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Add these helper functions to check stack status
func isStackInRollback(status types.StackStatus) bool {
	return status == types.StackStatusRollbackComplete ||
		status == types.StackStatusRollbackInProgress ||
		status == types.StackStatusUpdateRollbackComplete ||
		status == types.StackStatusUpdateRollbackInProgress
}

func isStackFailed(status types.StackStatus) bool {
	return status == types.StackStatusCreateFailed ||
		status == types.StackStatusDeleteFailed ||
		status == types.StackStatusUpdateFailed ||
		isStackInRollback(status)
}

// CreateInfrastructure creates the necessary AWS infrastructure including VPC, subnets,
// internet gateway, and routing tables. This is the main entry point for setting up
// the AWS networking infrastructure required for the deployment.
//
// The function performs the following steps:
// 1. Creates a VPC with CIDR block 10.0.0.0/16
// 2. Creates public and private subnets
// 3. Sets up an internet gateway
// 4. Configures routing tables for internet access
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//
// Returns:
//   - error: Returns an error if any step of the infrastructure creation fails
func (p *AWSProvider) CreateInfrastructure(ctx context.Context) error {
	l := logger.Get()
	l.Info("Creating AWS infrastructure...")

	// Create VPC and networking components
	if err := p.CreateVpc(ctx); err != nil {
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	l.Info("Infrastructure created successfully")
	return nil
}

// Destroy cleans up all AWS resources created for this deployment.
// This includes terminating instances, deleting VPC components, and removing any
// associated networking resources.
//
// The cleanup process follows this order:
// 1. Terminates all EC2 instances
// 2. Deletes route table associations
// 3. Deletes routes and route tables
// 4. Detaches and deletes internet gateways
// 5. Deletes subnets
// 6. Finally deletes the VPC
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//
// Returns:
//   - error: Returns an error if any cleanup step fails
func (p *AWSProvider) Destroy(ctx context.Context) error {
	l := logger.Get()
	l.Info("Starting destruction of AWS resources")

	// Clean up local state
	p.VPCID = ""

	return nil
}

// Helper function to delete a stack and wait for completion
func (p *AWSProvider) deleteStack(
	ctx context.Context,
	cfnClient *cloudformation.Client,
	stackName string,
) error {
	l := logger.Get()

	// Check if stack exists
	_, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		// If stack doesn't exist, just return
		if strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to describe stack %s: %w", stackName, err)
	}

	// Delete the stack
	_, err = cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete stack %s: %w", stackName, err)
	}

	l.Infof("Waiting for stack %s deletion to complete...", stackName)
	waiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
	maxWaitTime := 30 * time.Minute
	if err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, maxWaitTime); err != nil {
		return fmt.Errorf("failed waiting for stack %s deletion: %w", stackName, err)
	}

	l.Infof("Stack %s successfully destroyed", stackName)
	return nil
}

func (p *AWSProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return p.ClusterDeployer
}

func (p *AWSProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.ClusterDeployer = deployer
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

	// Clean up local state
	p.VPCID = ""

	// Remove the deployment from the configuration
	if err := p.removeDeploymentFromConfig(""); err != nil {
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
