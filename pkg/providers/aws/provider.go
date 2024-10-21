package awsprovider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf_types "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	aws_data "github.com/bacalhau-project/andaime/internal/clouds/aws"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	awsinterfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	ResourcePollingInterval = 10 * time.Second
	UpdateQueueSize         = 100
)

type VpcCdkStackProps struct {
	awscdk.StackProps
}

type AWSProvider struct {
	Config          *aws.Config
	EC2Client       awsinterfaces.EC2Clienter
	Region          string
	ClusterDeployer common_interface.ClusterDeployerer
	UpdateQueue     chan display.UpdateAction
	VPCID           string
	SubnetID        string
}

var ubuntuAMICache = make(map[string]string)
var cacheLock sync.RWMutex

// Add this near the top of the file, after the imports
type AWSConfig struct {
	StackName              string
	SubnetMask             int
	VpcCidr                string
	MaxAzs                 int
	CurrentDeploymentStage string
}

func NewAWSProvider(v *viper.Viper) (awsinterfaces.AWSProviderer, error) {
	ctx := context.Background()
	region := v.GetString("aws.region")
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	ec2Client, err := NewEC2Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client: %w", err)
	}

	awsProvider := &AWSProvider{
		Config:          &awsConfig,
		EC2Client:       ec2Client,
		Region:          region,
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAWS),
		UpdateQueue:     make(chan display.UpdateAction, UpdateQueueSize),
	}

	return awsProvider, nil
}

func (p *AWSProvider) PrepareDeployment(ctx context.Context) (*models.Deployment, error) {
	deployment, err := common.PrepareDeployment(ctx, models.DeploymentTypeAWS)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deployment: %w", err)
	}

	return deployment, nil
}

func (p *AWSProvider) ProcessMachinesConfig(
	ctx context.Context,
) (map[string]models.Machiner, map[string]bool, error) {
	l := logger.Get()
	machines := make(map[string]models.Machiner)
	locations := make(map[string]bool)

	machinesConfig := viper.GetStringMap("machines")
	if len(machinesConfig) == 0 {
		return nil, nil, fmt.Errorf("no machines configuration found for provider aws")
	}

	for machineName, machineConfig := range machinesConfig {
		config, ok := machineConfig.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("invalid machine configuration for %s", machineName)
		}

		provider, ok := config["provider"].(string)
		if !ok || provider != "aws" {
			continue
		}

		location, ok := config["location"].(string)
		if !ok || location == "" {
			return nil, nil, fmt.Errorf("location is required for AWS machine %s", machineName)
		}

		instanceType, ok := config["instance_type"].(string)
		if !ok || instanceType == "" {
			return nil, nil, fmt.Errorf("instance_type is required for AWS machine %s", machineName)
		}

		// Validate the machine type
		valid, err := p.ValidateMachineType(ctx, location, instanceType)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"error validating machine type for %s: %w",
				machineName,
				err,
			)
		}
		if !valid {
			return nil, nil, fmt.Errorf(
				"invalid machine type %s for location %s",
				instanceType,
				location,
			)
		}

		spotMarketOptions := &ec2_types.InstanceMarketOptionsRequest{
			MarketType: ec2_types.MarketTypeSpot,
			SpotOptions: &ec2_types.SpotMarketOptions{
				InstanceInterruptionBehavior: ec2_types.InstanceInterruptionBehaviorTerminate,
				SpotInstanceType:             ec2_types.SpotInstanceTypeOneTime,
			},
		}

		machine, err := models.NewMachine(
			models.DeploymentTypeAWS,
			machineName,
			instanceType,
			1,
			models.CloudSpecificInfo{
				Region:            location,
				SpotMarketOptions: spotMarketOptions,
			},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create machine: %w", err)
		}

		machines[machineName] = machine
		locations[location] = true

		l.Info(fmt.Sprintf("Processed machine configuration for %s: %+v", machineName, machine))
	}

	if len(machines) == 0 {
		return nil, nil, fmt.Errorf("no valid AWS machines found in configuration")
	}

	return machines, locations, nil
}

func (p *AWSProvider) StartResourcePolling(ctx context.Context) <-chan error {
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)

		ticker := time.NewTicker(ResourcePollingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := p.PollResources(ctx)
				if err != nil {
					errChan <- fmt.Errorf("failed to poll resources: %w", err)
				}
			}
		}
	}()
	return errChan
}

func (p *AWSProvider) PollResources(ctx context.Context) ([]interface{}, error) {
	instances, err := p.describeInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	resources := make([]interface{}, len(instances))
	for i, instance := range instances {
		resources[i] = instance
	}

	return resources, nil
}

func (p *AWSProvider) CreateVPCAndSubnet(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	deployment := m.Deployment
	if deployment.AWS == nil {
		return fmt.Errorf("deployment AWS is nil")
	}

	subnetID := deployment.AWS.SubnetID
	vpcID := deployment.AWS.VPCID

	if subnetID == "" || vpcID == "" {
		return fmt.Errorf("VPC ID or Subnet ID is not set")
	}

	// Get the account ID from environment variables
	accountID := os.Getenv("AWS_ACCOUNT_ID")
	if accountID == "" {
		return fmt.Errorf("environment variable AWS_ACCOUNT_ID is not set")
	}

	if p.Region == "" {
		return fmt.Errorf("environment variable AWS_REGION is not set")
	}

	// Create a new CDK app
	app := awscdk.NewApp(nil)

	// Create a stack
	stack := awscdk.NewStack(app, jsii.String(deployment.Name), &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(accountID),
			Region:  jsii.String(p.Region),
		},
	})

	// Create VPC
	vpc := awsec2.NewVpc(stack, jsii.String("AndaimeVPC"), &awsec2.VpcProps{
		MaxAzs:      jsii.Number(2),
		NatGateways: jsii.Number(1),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				CidrMask:   jsii.Number(24),
				Name:       jsii.String("Public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
			{
				CidrMask:   jsii.Number(24),
				Name:       jsii.String("Private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_NAT,
			},
		},
	})

	// Output VPC ID and Subnet IDs
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

	// Synthesize the stack to a CloudFormation template
	cloudFormationTemplate := app.Synth(nil).
		GetStackArtifact(jsii.String(deployment.Name)).
		Template()

	// Create CloudFormation client
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	cfnClient := cloudformation.NewFromConfig(cfg)

	// Create the CloudFormation stack
	stackName := fmt.Sprintf("%s-VPC-Stack", deployment.Name)
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(cloudFormationTemplate.(string)),
		Capabilities: []cf_types.Capability{cf_types.CapabilityCapabilityIam},
	})
	if err != nil {
		return fmt.Errorf("failed to create CloudFormation stack: %w", err)
	}

	// Wait for the stack to be created
	waiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)
	maxWaitTime := 30 * time.Minute
	if err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, maxWaitTime); err != nil {
		return fmt.Errorf("failed waiting for stack creation: %w", err)
	}

	// Describe the stack to get the outputs
	describeStackOutput, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(describeStackOutput.Stacks) == 0 {
		return fmt.Errorf("no stack found with name %s", stackName)
	}

	// Process the outputs and update the deployment
	for _, output := range describeStackOutput.Stacks[0].Outputs {
		switch *output.OutputKey {
		case "VpcId":
			deployment.AWS.VPCID = *output.OutputValue
		case "PublicSubnet0":
			deployment.AWS.SubnetID = *output.OutputValue
		}
	}

	return nil
}

func (p *AWSProvider) CreateResources(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	for _, machine := range m.Deployment.Machines {
		err := p.createInstance(ctx, machine, m.Deployment.AWS.SubnetID)
		if err != nil {
			return fmt.Errorf(
				"failed to create instance for machine %s: %w",
				machine.GetName(),
				err,
			)
		}
	}

	return nil
}

func (p *AWSProvider) createInstance(
	ctx context.Context,
	machine models.Machiner,
	subnetID string,
) error {
	image, err := p.GetLatestUbuntuImage(ctx, p.Region)
	if err != nil {
		return fmt.Errorf("failed to get latest Ubuntu image: %w", err)
	}

	machine.SetImageID(*image.ImageId)

	runInstancesInput := p.createEC2InstanceInput(machine, subnetID)

	result, err := p.EC2Client.RunInstances(ctx, runInstancesInput)
	if err != nil || result == nil || len(result.Instances) == 0 {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	machine.SetMachineResourceState(
		models.AWSResourceTypeInstance.ResourceString,
		models.ResourceStateSucceeded,
	)

	return nil
}

func (p *AWSProvider) createEC2InstanceInput(
	machine models.Machiner,
	subnetID string,
) *ec2.RunInstancesInput {
	l := logger.Get()

	minCount := int32(1)
	maxCount := minCount

	l.Info("provisioning VMs", zap.Int32("desired_count", minCount))

	username := machine.GetSSHUser()
	sshPubKeyMaterial := string(machine.GetSSHPublicKeyMaterial())

	userData := fmt.Sprintf(`#!/bin/bash
SSH_USERNAME=%s
SSH_PUBLIC_KEY_MATERIAL="%s"

useradd -m -s /bin/bash $SSH_USERNAME
passwd -l $SSH_USERNAME
mkdir -p /home/$SSH_USERNAME/.ssh
echo "$SSH_PUBLIC_KEY_MATERIAL" > /root/root.pub
cat /root/root.pub >> /home/$SSH_USERNAME/.ssh/authorized_keys

chown -R $SSH_USERNAME:$SSH_USERNAME /home/$SSH_USERNAME/.ssh
chmod 700 /home/$SSH_USERNAME/.ssh
chmod 600 /home/$SSH_USERNAME/.ssh/authorized_keys
usermod -aG sudo $SSH_USERNAME

# Add $SSH_USERNAME to the sudoers file
echo "$SSH_USERNAME ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers`,
		username,
		sshPubKeyMaterial,
	)

	// Encode the script in base64
	encodedUserData := base64.StdEncoding.EncodeToString([]byte(userData))

	return &ec2.RunInstancesInput{
		ImageId:      aws.String(machine.GetImageID()),
		InstanceType: ec2_types.InstanceType(machine.GetVMSize()),
		MinCount:     aws.Int32(minCount),
		MaxCount:     aws.Int32(maxCount),
		SubnetId:     aws.String(subnetID),
		InstanceMarketOptions: &ec2_types.InstanceMarketOptionsRequest{
			MarketType: ec2_types.MarketTypeSpot,
			SpotOptions: &ec2_types.SpotMarketOptions{
				InstanceInterruptionBehavior: ec2_types.InstanceInterruptionBehaviorTerminate,
				SpotInstanceType:             ec2_types.SpotInstanceTypeOneTime,
			},
		},
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeInstance,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(machine.GetName()),
					},
					{
						Key:   aws.String("Architecture"),
						Value: aws.String("x86_64"),
					},
				},
			},
		},
		UserData: aws.String(encodedUserData),
	}
}

func (p *AWSProvider) createSpotInstanceInput(
	imageID *string,
	instanceSize string,
) *ec2_types.InstanceMarketOptionsRequest {
	return &ec2_types.InstanceMarketOptionsRequest{
		MarketType: ec2_types.MarketTypeSpot,
		SpotOptions: &ec2_types.SpotMarketOptions{
			MaxPrice: aws.String("1.00"),
		},
	}
}

func (p *AWSProvider) GetClusterDeployer() common_interface.ClusterDeployerer {
	return p.ClusterDeployer
}

func (p *AWSProvider) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.ClusterDeployer = deployer
}

func (p *AWSProvider) FinalizeDeployment(ctx context.Context) error {
	// Implement any final steps needed for the deployment
	return nil
}

// ConfigInterface defines the interface for configuration operations
type ConfigInterfacer interface {
	GetString(key string) string
}

// Destroy destroys the AWSProvider instance
func (p *AWSProvider) Destroy(ctx context.Context) error {
	return nil
}

// ConfigWrapper wraps the AWS config to implement ConfigInterface
type ConfigWrapper struct {
	config aws.Config
}

func NewConfigWrapper(config aws.Config) *ConfigWrapper {
	return &ConfigWrapper{config: config}
}

func (cw *ConfigWrapper) GetString(key string) string {
	// Implement this method based on how you're storing/retrieving config values
	// This is just a placeholder
	return ""
}

// GetEC2Client returns the current EC2 client
func (p *AWSProvider) GetEC2Client() (awsinterfaces.EC2Clienter, error) {
	return p.EC2Client, nil
}

// SetEC2Client sets a new EC2 client
func (p *AWSProvider) SetEC2Client(client awsinterfaces.EC2Clienter) {
	p.EC2Client = client
}

// CreateDeployment performs the AWS deployment
func (p *AWSProvider) CreateDeployment(
	ctx context.Context,
) error {
	if err := p.CreateVPCAndSubnet(ctx); err != nil {
		return fmt.Errorf("failed to create VPC and subnet: %w", err)
	}

	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	image, err := p.GetLatestUbuntuImage(ctx, p.Region)
	if err != nil {
		return fmt.Errorf("failed to get latest Ubuntu image: %w", err)
	}

	l.Infof("Latest Ubuntu AMI ID for region %s: %s\n", p.Region, *image.ImageId)

	var runInstancesInput *ec2.RunInstancesInput

	result, err := p.EC2Client.RunInstances(ctx, runInstancesInput)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	if len(result.Instances) == 0 {
		return fmt.Errorf("no instances created")
	}

	l.Infof("Created instance: %s\n", *result.Instances[0].InstanceId)
	return nil
}

func NewVpcStack(
	scope constructs.Construct,
	id string,
	props *VpcCdkStackProps,
	config *AWSConfig,
) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here
	ngwNum := 0
	subnetConfigs := []*awsec2.SubnetConfiguration{
		{
			Name:                jsii.String("PublicSubnet"),
			MapPublicIpOnLaunch: jsii.Bool(true),
			SubnetType:          awsec2.SubnetType_PUBLIC,
			CidrMask:            jsii.Number(float64(config.SubnetMask)),
		},
	}

	if config.CurrentDeploymentStage == "PROD" {
		ngwNum = config.MaxAzs
		privateSub := &awsec2.SubnetConfiguration{
			Name:       jsii.String("PrivateSubnet"),
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_NAT,
			CidrMask:   jsii.Number(float64(config.SubnetMask)),
		}
		subnetConfigs = append(subnetConfigs, privateSub)
	}

	vpc := awsec2.NewVpc(stack, jsii.String("Vpc"), &awsec2.VpcProps{
		VpcName:             jsii.String(*stack.StackName() + "-Vpc"),
		Cidr:                jsii.String(config.VpcCidr),
		EnableDnsHostnames:  jsii.Bool(true),
		EnableDnsSupport:    jsii.Bool(true),
		MaxAzs:              jsii.Number(float64(config.MaxAzs)),
		NatGateways:         jsii.Number(float64(ngwNum)),
		SubnetConfiguration: &subnetConfigs,
	})

	// Tagging subnets
	// Public subnets
	for index, subnet := range *vpc.PublicSubnets() {
		subnetName := *stack.StackName() + "-PublicSubnet0" + strconv.Itoa(index+1)
		awscdk.Tags_Of(subnet).Add(jsii.String("Name"), jsii.String(subnetName), &awscdk.TagProps{})
	}
	// Private subnets
	for index, subnet := range *vpc.PrivateSubnets() {
		subnetName := *stack.StackName() + "-PrivateSubnet0" + strconv.Itoa(index+1)
		awscdk.Tags_Of(subnet).Add(jsii.String("Name"), jsii.String(subnetName), &awscdk.TagProps{})
	}

	return stack
}

func (p *AWSProvider) describeInstances(ctx context.Context) ([]*ec2_types.Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	result, err := p.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []*ec2_types.Instance
	for _, reservation := range result.Reservations {
		for i := range reservation.Instances {
			instances = append(instances, &reservation.Instances[i])
		}
	}

	return instances, nil
}

func (p *AWSProvider) ListDeployments(ctx context.Context) ([]*ec2_types.Instance, error) {
	instances, err := p.describeInstances(ctx)
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (p *AWSProvider) terminateInstances(ctx context.Context, instanceIDs []string) error {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	}
	_, err := p.EC2Client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instances: %w", err)
	}
	return nil
}

func (p *AWSProvider) TerminateDeployment(ctx context.Context) error {
	instances, err := p.describeInstances(ctx)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	var instanceIDs []string
	for _, instance := range instances {
		instanceIDs = append(instanceIDs, *instance.InstanceId)
	}

	if len(instanceIDs) > 0 {
		err = p.terminateInstances(ctx, instanceIDs)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetLatestUbuntuImage gets the latest Ubuntu AMI for the specified region
func (p *AWSProvider) GetLatestUbuntuImage(
	ctx context.Context,
	region string,
) (*ec2_types.Image, error) {
	cacheLock.RLock()
	cachedAMI, found := ubuntuAMICache[p.Region]
	cacheLock.RUnlock()

	if found {
		return &ec2_types.Image{ImageId: aws.String(cachedAMI)}, nil
	}

	input := &ec2.DescribeImagesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
			{
				Name:   aws.String("root-device-type"),
				Values: []string{"ebs"},
			},
			{
				Name:   aws.String("virtualization-type"),
				Values: []string{"hvm"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("owner-id"),
				Values: []string{"099720109477"}, // Canonical's AWS account ID
			},
		},
	}

	result, err := p.EC2Client.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe images: %w", err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("no Ubuntu images found")
	}

	var latestImage *ec2_types.Image
	var latestTime time.Time

	for _, image := range result.Images {
		internalImage := image
		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			continue
		}

		if latestImage == nil || creationTime.After(latestTime) {
			latestImage = &internalImage
			latestTime = creationTime
		}
	}

	if latestImage == nil {
		return nil, fmt.Errorf("failed to find latest Ubuntu image")
	}

	cacheLock.Lock()
	ubuntuAMICache[p.Region] = *latestImage.ImageId
	cacheLock.Unlock()

	return latestImage, nil
}

func (p *AWSProvider) GetVMExternalIP(ctx context.Context, instanceID string) (string, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}
	result, err := p.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe instances: %w", err)
	}
	return *result.Reservations[0].Instances[0].PublicIpAddress, nil
}

func (p *AWSProvider) ValidateMachineType(
	ctx context.Context,
	location, instanceType string,
) (bool, error) {
	valid := aws_data.IsValidAWSInstanceType(location, instanceType)

	if !valid {
		return false, fmt.Errorf("invalid machine type: %s", instanceType)
	}

	return valid, nil
}
