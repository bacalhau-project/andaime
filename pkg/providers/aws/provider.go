package aws

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	internal_aws "github.com/bacalhau-project/andaime/internal/clouds/aws"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

const (
	ResourcePollingInterval = 30 * time.Second
	UpdateQueueSize         = 1000 // Increased from 100 to prevent dropping updates
	UpdatePollingInterval   = 100 * time.Millisecond
	DefaultStackTimeout     = 30 * time.Minute
	TestStackTimeout        = 30 * time.Second
	UbuntuAMIOwner          = "099720109477" // Canonical's AWS account ID
)

type DeploymentInfo struct {
	ID            string
	Region        string
	VPCCount      int
	InstanceCount int
	Tags          map[string]string
}

// String returns a formatted string representation of the deployment info
func (d DeploymentInfo) String() string {
	var tags []string
	for k, v := range d.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(tags) // Sort tags for consistent output
	tagStr := ""
	if len(tags) > 0 {
		tagStr = fmt.Sprintf(" [%s]", strings.Join(tags, ", "))
	}
	return fmt.Sprintf("Deployment %s (Region: %s, VPCs: %d, Instances: %d)%s",
		d.ID, d.Region, d.VPCCount, d.InstanceCount, tagStr)
}

// AWSProvider updated structure
type AWSProvider struct {
	AccountID       string
	Config          *aws.Config
	ClusterDeployer common_interface.ClusterDeployerer
	UpdateQueue     chan display.UpdateAction
	EC2Client       aws_interface.EC2Clienter
	STSClient       *sts.Client
	ConfigMutex     sync.RWMutex
}

var NewAWSProviderFunc = NewAWSProvider

// NewAWSProvider creates a new AWS provider instance
func NewAWSProvider(accountID string) (*AWSProvider, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	// Load default AWS config without region
	awsConfig, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	provider := &AWSProvider{
		AccountID:       accountID,
		Config:          &awsConfig,
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAWS),
		UpdateQueue:     make(chan display.UpdateAction, UpdateQueueSize),
		STSClient:       sts.NewFromConfig(awsConfig),
	}

	// Initialize a default EC2 client - region doesn't matter here as we'll create regional clients as needed
	ec2Client := ec2.NewFromConfig(awsConfig)
	provider.EC2Client = &LiveEC2Client{client: ec2Client}

	return provider, nil
}

// getOrCreateEC2Client gets or creates an EC2 client for a specific region
func (p *AWSProvider) getOrCreateEC2Client(
	ctx context.Context,
	region string,
) (aws_interface.EC2Clienter, error) {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return nil, fmt.Errorf("global model or deployment is nil")
	}

	if m.Deployment.AWS.RegionalResources.Clients == nil {
		m.Deployment.AWS.RegionalResources.Clients = make(map[string]aws_interface.EC2Clienter)
	}

	if m.Deployment.AWS.RegionalResources.Clients[region] != nil {
		return m.Deployment.AWS.RegionalResources.Clients[region], nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
	}

	var ec2Client aws_interface.EC2Clienter
	if p.GetEC2Client() != nil {
		// If we already have a global EC2 client, use it
		ec2Client = p.GetEC2Client()
	} else {
		ec2Client = &LiveEC2Client{client: ec2.NewFromConfig(cfg)}
	}
	return ec2Client, nil
}

func (p *AWSProvider) PrepareDeployment(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// Get deployment and ensure it has AWS config
	deployment := m.Deployment

	if deployment.AWS == nil {
		deployment.AWS = &models.AWSDeployment{
			RegionalResources: &models.RegionalResources{
				VPCs:    make(map[string]*models.AWSVPC),
				Clients: make(map[string]aws_interface.EC2Clienter),
			},
		}
	}

	// Get AWS account ID if not already set
	if deployment.AWS.AccountID == "" {
		identity, err := p.STSClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return fmt.Errorf("failed to get AWS account ID: %w", err)
		}
		deployment.AWS.AccountID = *identity.Account
	}

	// Initialize regional resources
	regions := make(map[string]struct{})
	for _, machine := range deployment.Machines {
		region := machine.GetLocation()
		if region == "" {
			continue
		}
		// Convert zone to region if necessary
		if len(region) > 0 && region[len(region)-1] >= 'a' && region[len(region)-1] <= 'z' {
			region = region[:len(region)-1]
		}
		regions[region] = struct{}{}
	}

	// Create VPC and security group for each region
	for region := range regions {
		if _, exists := deployment.AWS.RegionalResources.VPCs[region]; !exists {
			vpc, err := p.createVPCInfrastructure(ctx, region)
			if err != nil {
				return fmt.Errorf(
					"failed to create VPC infrastructure in region %s: %w",
					region,
					err,
				)
			}
			deployment.AWS.RegionalResources.VPCs[region] = vpc
		}
	}

	return nil
}

func (p *AWSProvider) createVPCInfrastructure(
	ctx context.Context,
	region string,
) (*models.AWSVPC, error) {
	// Create EC2 client for the region
	ec2Client, err := p.getOrCreateEC2Client(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client for region %s: %w", region, err)
	}

	// Create VPC with CIDR block
	vpcOut, err := ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeVpc,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("andaime-vpc"),
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	// Create security group
	sgOut, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("andaime-sg"),
		Description: aws.String("Security group for Andaime deployment"),
		VpcId:       vpcOut.Vpc.VpcId,
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeSecurityGroup,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("andaime-sg"),
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}

	// Allow inbound traffic
	_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: sgOut.GroupId,
		IpPermissions: []ec2_types.IpPermission{
			{
				IpProtocol: aws.String("-1"),
				FromPort:   aws.Int32(-1),
				ToPort:     aws.Int32(-1),
				IpRanges: []ec2_types.IpRange{
					{
						CidrIp: aws.String("0.0.0.0/0"),
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to authorize security group ingress: %w", err)
	}

	return &models.AWSVPC{
		VPCID:           *vpcOut.Vpc.VpcId,
		SecurityGroupID: *sgOut.GroupId,
	}, nil
}

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
		resourceTicker := time.NewTicker(ResourcePollingInterval)
		updateTicker := time.NewTicker(UpdatePollingInterval)
		defer resourceTicker.Stop()
		defer updateTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-resourceTicker.C:
				if err := p.pollResources(ctx); err != nil {
					logger.Get().Error(fmt.Sprintf("Failed to poll resources: %v", err))
				}
			case <-updateTicker.C:
				p.processUpdateQueue()
			}
		}
	}()
	return nil
}

func (p *AWSProvider) processUpdateQueue() {
	m := display.GetGlobalModelFunc()
	if m == nil {
		return
	}

	// Process up to 100 updates per tick to prevent queue from filling
	for i := 0; i < 100; i++ {
		select {
		case update := <-p.UpdateQueue:
			m.QueueUpdate(update)
		default:
			return // No more updates in queue
		}
	}
}

func (p *AWSProvider) pollResources(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("display model or deployment is nil")
	}

	// Create EC2 client
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Config.Region))
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
			machineID := getMachineIDFromTags(instance.Tags)
			if machineID != "" {
				machine := m.Deployment.GetMachine(machineID)
				if machine == nil {
					continue
				}

				// Check and update instance state
				status := mapEC2StateToMachineState(instance.State.Name)
				p.updateMachineStatus(machineID, status)

				// Check and update network interface state
				if len(instance.NetworkInterfaces) > 0 {
					networkStatus := models.ResourceStateRunning
					if instance.NetworkInterfaces[0].Status != "in-use" {
						networkStatus = models.ResourceStatePending
					}
					m.QueueUpdate(display.UpdateAction{
						MachineName: machine.GetName(),
						UpdateData: display.UpdateData{
							UpdateType:    display.UpdateTypeResource,
							ResourceType:  "Network",
							ResourceState: networkStatus,
						},
					})
				}

				// Check and update volume state
				if len(instance.BlockDeviceMappings) > 0 {
					volumeStatus := models.ResourceStateRunning
					if instance.BlockDeviceMappings[0].Ebs != nil &&
						instance.BlockDeviceMappings[0].Ebs.Status != "attached" {
						volumeStatus = models.ResourceStatePending
					}
					m.QueueUpdate(display.UpdateAction{
						MachineName: machine.GetName(),
						UpdateData: display.UpdateData{
							UpdateType:    display.UpdateTypeResource,
							ResourceType:  "Volume",
							ResourceState: volumeStatus,
						},
					})
				}
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

// CreateVpc creates a new VPC with the specified CIDR block
func (p *AWSProvider) CreateVpc(ctx context.Context, region string) error {
	l := logger.Get()
	l.Debugf("Creating VPC in region %s", region)

	// Get or create EC2 client for this region
	regionalClient, err := p.getOrCreateEC2Client(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to get EC2 client: %w", err)
	}

	createVpcInput := &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeVpc,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("andaime-vpc"),
					},
					{
						Key:   aws.String("andaime"),
						Value: aws.String("true"),
					},
				},
			},
		},
	}

	l.Debugf("Sending CreateVpc request with cidr_block %s", "10.0.0.0/16")

	createVpcOutput, err := regionalClient.CreateVpc(ctx, createVpcInput)
	if err != nil {
		l.Debugf("VPC creation failed: %v", err)
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	vpcID := *createVpcOutput.Vpc.VpcId
	l.Debugf("Created VPC with ID %s", vpcID)

	// Wait for VPC to become available
	l.Info("Waiting for VPC to be available...")
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute
	b.InitialInterval = 2 * time.Second
	b.MaxInterval = 30 * time.Second

	operation := func() error {
		if err := ctx.Err(); err != nil {
			return backoff.Permanent(err)
		}

		input := &ec2.DescribeVpcsInput{
			VpcIds: []string{vpcID},
		}

		result, err := regionalClient.DescribeVpcs(ctx, input)
		if err != nil {
			l.Debugf("Failed to describe VPC with ID %s: %v", vpcID, err)
			return fmt.Errorf("failed to describe VPC: %w", err)
		}

		if len(result.Vpcs) > 0 {
			l.Debugf("VPC status check with ID %s: %s", vpcID, string(result.Vpcs[0].State))

			if result.Vpcs[0].State == ec2_types.VpcStateAvailable {
				l.Debugf("VPC is now available with ID %s", vpcID)
				return nil
			}
		}

		l.Debugf("VPC not yet available with ID %s", vpcID)
		return fmt.Errorf("VPC not yet available")
	}

	if err := backoff.Retry(operation, backoff.WithContext(b, ctx)); err != nil {
		return fmt.Errorf("timeout waiting for VPC to become available: %w", err)
	}

	m := display.GetGlobalModelFunc()
	if m != nil && m.Deployment != nil {
		for _, machine := range m.Deployment.GetMachines() {
			m.QueueUpdate(display.UpdateAction{
				MachineName: machine.GetName(),
				UpdateData: display.UpdateData{
					UpdateType:    display.UpdateTypeResource,
					ResourceType:  "VPC",
					ResourceState: models.ResourceStatePending,
				},
			})
		}
	}

	// Enable DNS hostnames
	modifyVpcAttributeInput := &ec2.ModifyVpcAttributeInput{
		VpcId:              aws.String(vpcID),
		EnableDnsHostnames: &ec2_types.AttributeBooleanValue{Value: aws.Bool(true)},
	}

	_, err = regionalClient.ModifyVpcAttribute(ctx, modifyVpcAttributeInput)
	if err != nil {
		return fmt.Errorf("failed to enable DNS hostnames: %w", err)
	}

	// Update all machines to show VPC is ready
	if m != nil && m.Deployment != nil {
		for _, machine := range m.Deployment.GetMachines() {
			m.QueueUpdate(display.UpdateAction{
				MachineName: machine.GetName(),
				UpdateData: display.UpdateData{
					UpdateType:    display.UpdateTypeResource,
					ResourceType:  "VPC",
					ResourceState: models.ResourceStateRunning,
				},
			})
		}
	}

	// Save VPC ID to config immediately after creation
	if model := display.GetGlobalModelFunc(); model != nil && model.Deployment != nil {
		deploymentPath := fmt.Sprintf("deployments.%s", model.Deployment.UniqueID)
		viper.Set(fmt.Sprintf("%s.aws.regions.%s.vpc_id",
			deploymentPath, region),
			m.Deployment.AWS.RegionalResources.VPCs[region].VPCID)
		if err := viper.WriteConfig(); err != nil {
			l.Warnf("Failed to save VPC ID to config: %v", err)
		} else {
			l.Debugf("Saved VPC ID %s to config at %s.aws.vpc_id",
				m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
				deploymentPath)
		}
	}

	l.Debugf("Regional VPC (Region: %s) created successfully with ID %s",
		region, m.Deployment.AWS.RegionalResources.VPCs[region].VPCID)

	// Save VPC ID to config immediately after creation
	if m != nil && m.Deployment != nil {
		deploymentPath := fmt.Sprintf("deployments.%s.aws", m.Deployment.UniqueID)
		viper.Set(
			fmt.Sprintf("%s.vpc_id", deploymentPath),
			m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
		)
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to save VPC ID to config: %w", err)
		}
		l.Infof(
			"Saved VPC ID %s to config at %s.vpc_id",
			m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
			deploymentPath,
		)
	}

	return nil
}

func (p *AWSProvider) CreateInfrastructure(ctx context.Context) error {
	l := logger.Get()
	l.Info("Creating AWS infrastructure...")

	allRegions, err := p.GetAllAWSRegions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all AWS regions: %w", err)
	}

	m := display.GetGlobalModelFunc()
	if m != nil && m.Deployment != nil {
		for _, machine := range m.Deployment.GetMachines() {
			m.QueueUpdate(display.UpdateAction{
				MachineName: machine.GetName(),
				UpdateData: display.UpdateData{
					UpdateType:    display.UpdateTypeResource,
					ResourceType:  "Infrastructure",
					ResourceState: models.ResourceStatePending,
				},
			})
		}
	}

	var eg errgroup.Group

	for _, region := range allRegions {
		region := region
		eg.Go(func() error {
			// Fetch available AZs for the region
			l.Info(fmt.Sprintf("Fetching availability zones for region %s...", region))
			azInput := &ec2.DescribeAvailabilityZonesInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("region-name"),
						Values: []string{region},
					},
					{
						Name:   aws.String("state"),
						Values: []string{"available"},
					},
				},
			}

			azOutput, err := p.EC2Client.DescribeAvailabilityZones(ctx, azInput)
			if err != nil {
				return fmt.Errorf("failed to get availability zones for region %s: %w", region, err)
			}

			if len(azOutput.AvailabilityZones) < 2 {
				return fmt.Errorf("region %s does not have at least 2 availability zones", region)
			}

			// Create VPC with retry on limits exceeded
			if err := p.createVPCWithRetry(ctx, region); err != nil {
				return fmt.Errorf("failed to create VPC: %w", err)
			}

			l.Info("Create Security Groups...")
			sgID, err := p.createSecurityGroup(
				ctx,
				m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
			)
			if err != nil {
				return fmt.Errorf("failed to create security groups: %w", err)
			}
			m.Deployment.AWS.RegionalResources.VPCs[region].SecurityGroupID = *sgID

			// Create minimum required subnets (1 public, 1 private) in two AZs for high availability
			l.Info("Creating optimized subnet configuration...")
			azCount := 2 // Minimum recommended for high availability

			for i := 0; i < azCount; i++ {
				azName := *azOutput.AvailabilityZones[i].ZoneName

				// Create public subnet
				publicSubnetInput := &ec2.CreateSubnetInput{
					VpcId: aws.String(
						m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
					),
					AvailabilityZone: aws.String(azName),
					CidrBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", i*2)),
					TagSpecifications: []ec2_types.TagSpecification{
						{
							ResourceType: ec2_types.ResourceTypeSubnet,
							Tags: []ec2_types.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String(fmt.Sprintf("public-subnet-%s", azName)),
								},
								{Key: aws.String("Type"), Value: aws.String("public")},
							},
						},
					},
				}
				publicSubnet, err := p.EC2Client.CreateSubnet(ctx, publicSubnetInput)
				if err != nil {
					return fmt.Errorf("failed to create public subnet in %s: %w", azName, err)
				}
				m.Deployment.AWS.RegionalResources.VPCs[region].PublicSubnetIDs = append(
					m.Deployment.AWS.RegionalResources.VPCs[region].PublicSubnetIDs,
					*publicSubnet.Subnet.SubnetId,
				)

				// Create private subnet
				privateSubnetInput := &ec2.CreateSubnetInput{
					VpcId: aws.String(
						m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
					),
					AvailabilityZone: aws.String(azName),
					CidrBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", i*2+1)),
					TagSpecifications: []ec2_types.TagSpecification{
						{
							ResourceType: ec2_types.ResourceTypeSubnet,
							Tags: []ec2_types.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String(fmt.Sprintf("private-subnet-%s", azName)),
								},
								{Key: aws.String("Type"), Value: aws.String("private")},
							},
						},
					},
				}
				privateSubnet, err := p.EC2Client.CreateSubnet(ctx, privateSubnetInput)
				if err != nil {
					return fmt.Errorf("failed to create private subnet in %s: %w", azName, err)
				}
				m.Deployment.AWS.RegionalResources.VPCs[region].PrivateSubnetIDs = append(
					m.Deployment.AWS.RegionalResources.VPCs[region].PrivateSubnetIDs,
					*privateSubnet.Subnet.SubnetId,
				)
			}

			// Rest of the infrastructure setup remains the same
			l.Info("Creating Internet Gateway...")
			igw, err := p.EC2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
				TagSpecifications: []ec2_types.TagSpecification{
					{
						ResourceType: ec2_types.ResourceTypeInternetGateway,
						Tags: []ec2_types.Tag{
							{Key: aws.String("Name"), Value: aws.String("andaime-igw")},
						},
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create internet gateway: %w", err)
			}
			m.Deployment.AWS.RegionalResources.VPCs[region].InternetGatewayID = *igw.InternetGateway.InternetGatewayId

			// Attach Internet Gateway to VPC
			l.Info("Attaching Internet Gateway to VPC...")
			_, err = p.EC2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
				InternetGatewayId: aws.String(
					m.Deployment.AWS.RegionalResources.VPCs[region].InternetGatewayID,
				),
				VpcId: aws.String(
					m.Deployment.AWS.RegionalResources.VPCs[region].VPCID,
				),
			})
			if err != nil {
				return fmt.Errorf("failed to attach internet gateway: %w", err)
			}

			// Create and configure route tables
			l.Info("Creating route tables...")
			publicRT, err := p.EC2Client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
				VpcId: aws.String(m.Deployment.AWS.RegionalResources.VPCs[region].VPCID),
				TagSpecifications: []ec2_types.TagSpecification{
					{
						ResourceType: ec2_types.ResourceTypeRouteTable,
						Tags: []ec2_types.Tag{
							{Key: aws.String("Name"), Value: aws.String("public-rt")},
						},
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create public route table: %w", err)
			}
			m.Deployment.AWS.RegionalResources.VPCs[region].PublicRouteTableID = *publicRT.RouteTable.RouteTableId

			// Create route to Internet Gateway
			_, err = p.EC2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
				RouteTableId: aws.String(
					m.Deployment.AWS.RegionalResources.VPCs[region].PublicRouteTableID,
				),
				DestinationCidrBlock: aws.String("0.0.0.0/0"),
				GatewayId: aws.String(
					m.Deployment.AWS.RegionalResources.VPCs[region].InternetGatewayID,
				),
			})
			if err != nil {
				return fmt.Errorf("failed to create route to internet gateway: %w", err)
			}

			// Associate public subnets with public route table
			for _, subnetID := range m.Deployment.AWS.RegionalResources.VPCs[region].PublicSubnetIDs {
				_, err = p.EC2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
					RouteTableId: aws.String(
						m.Deployment.AWS.RegionalResources.VPCs[region].PublicRouteTableID,
					),
					SubnetId: aws.String(subnetID),
				})
				if err != nil {
					return fmt.Errorf(
						"failed to associate public subnet %s with route table: %w",
						subnetID,
						err,
					)
				}
			}

			if err := p.saveInfrastructureToConfig(); err != nil {
				return fmt.Errorf("failed to save infrastructure IDs to config: %w", err)
			}

			// Update display
			if m != nil && m.Deployment != nil {
				for _, machine := range m.Deployment.GetMachines() {
					m.QueueUpdate(display.UpdateAction{
						MachineName: machine.GetName(),
						UpdateData: display.UpdateData{
							UpdateType:    display.UpdateTypeResource,
							ResourceType:  "Infrastructure",
							ResourceState: models.ResourceStateSucceeded,
						},
					})
				}
			}

			l.Info("AWS infrastructure created successfully")
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to create infrastructure: %w", err)
	}

	l.Info("Infrastructure created successfully")
	return nil
}

// createVPCWithRetry handles VPC creation with cleanup retry logic
func (p *AWSProvider) createVPCWithRetry(ctx context.Context, region string) error {
	if err := p.CreateVpc(ctx, region); err != nil {
		if strings.Contains(err.Error(), "VpcLimitExceeded") {
			// Try to find and clean up any abandoned VPCs
			if cleanupErr := p.cleanupAbandonedVPCs(ctx); cleanupErr != nil {
				return fmt.Errorf("failed to cleanup VPCs: %w", cleanupErr)
			}
			// Retry VPC creation after cleanup
			if retryErr := p.CreateVpc(ctx, region); retryErr != nil {
				return fmt.Errorf("failed to create VPC after cleanup: %w", retryErr)
			}
		} else {
			return err
		}
	}

	return nil
}

func (p *AWSProvider) createSecurityGroup(ctx context.Context,
	vpcid string) (*string, error) {
	l := logger.Get()
	l.Info("Creating security groups...")
	sgOutput, err := p.GetEC2Client().CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("andaime-sg"),
		VpcId:       aws.String(vpcid),
		Description: aws.String("Security group for Andaime deployments"),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeSecurityGroup,
				Tags: []ec2_types.Tag{
					{Key: aws.String("Name"), Value: aws.String("andaime-sg")},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}

	sgRules := []ec2_types.IpPermission{}
	allowedPorts := []int{22, 1234, 1235, 4222}
	for _, port := range allowedPorts {
		sgRules = append(sgRules, ec2_types.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(int32(port)),
			ToPort:     aws.Int32(int32(port)),
			IpRanges: []ec2_types.IpRange{
				{
					CidrIp: aws.String("0.0.0.0/0"),
				},
			},
		})
	}

	_, err = p.GetEC2Client().
		AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       sgOutput.GroupId,
			IpPermissions: sgRules,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to authorize security group ingress: %w", err)
	}

	return sgOutput.GroupId, nil
}

func (p *AWSProvider) importSSHKeyPair(
	_ context.Context,
	sshPublicKeyPath string,
) (string, error) {
	l := logger.Get()
	l.Info("Reading SSH public key...")

	// Read the public key file
	publicKeyBytes, err := os.ReadFile(sshPublicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH public key: %w", err)
	}

	l.Infof("Read SSH public key from %s", sshPublicKeyPath)
	return string(publicKeyBytes), nil
}

// generateRandomString creates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// saveInfrastructureToConfig saves all infrastructure IDs to the configuration
func (p *AWSProvider) saveInfrastructureToConfig() error {
	// Create a local lock to ensure we only write to the config file once
	p.ConfigMutex.Lock()
	defer p.ConfigMutex.Unlock()

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	basePath := m.Deployment.ViperPath
	for region, vpc := range m.Deployment.AWS.RegionalResources.VPCs {
		viper.Set(fmt.Sprintf("%s.regions.%s.vpc_id", basePath, region), vpc.VPCID)
	}

	return viper.WriteConfig()
}

func (p *AWSProvider) WaitForNetworkConnectivity(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	var eg errgroup.Group
	for region := range m.Deployment.AWS.RegionalResources.VPCs {
		region := region
		vpcID := m.Deployment.AWS.RegionalResources.VPCs[region].VPCID

		eg.Go(func() error {
			l := logger.Get()
			l.Debugf("Starting network connectivity check with VPC ID %s", vpcID)

			b := backoff.NewExponentialBackOff()
			b.InitialInterval = 5 * time.Second
			b.MaxInterval = 30 * time.Second
			b.MaxElapsedTime = 5 * time.Minute

			l.Info("Starting network connectivity check...")
			l.Infof("Using VPC ID: %s", vpcID)

			operation := func() error {
				if err := ctx.Err(); err != nil {
					l.Error(fmt.Sprintf("Context error: %v", err))
					return backoff.Permanent(err)
				}

				// Check if we can describe route tables (tests network connectivity)
				input := &ec2.DescribeRouteTablesInput{
					Filters: []ec2_types.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []string{vpcID},
						},
					},
				}

				l.Info("Attempting to describe route tables for VPC connectivity check...")
				l.Debugf("Looking for route tables in VPC %s with internet gateway routes", vpcID)
				result, err := p.EC2Client.DescribeRouteTables(ctx, input)
				if err != nil {
					l.Debugf("Failed to describe route tables: %v", err)
					return fmt.Errorf("failed to describe route tables: %w", err)
				}

				l.Debugf("Route tables found with VPC ID %s: %d", vpcID, len(result.RouteTables))

				// Verify route table has internet gateway route
				hasInternetRoute := false
				for i, rt := range result.RouteTables {
					l.Infof("Checking route table %d...", i+1)
					l.Infof("Route table ID: %s", *rt.RouteTableId)

					for _, route := range rt.Routes {
						if route.GatewayId != nil {
							l.Debugf(
								"Examining route: [RouteTable: %s] [GatewayID: %s] [Destination: %s] [State: %s]",
								*rt.RouteTableId,
								aws.ToString(route.GatewayId),
								aws.ToString(route.DestinationCidrBlock),
								string(route.State),
							)
							if strings.HasPrefix(*route.GatewayId, "igw-") {
								hasInternetRoute = true
								l.Info("Found internet gateway route!")
								break
							}
						}
					}
				}

				if !hasInternetRoute {
					l.Warn("No internet gateway route found in any route table")
					return fmt.Errorf("internet gateway route not found")
				}

				l.Info("Network connectivity check passed")
				return nil
			}

			return backoff.Retry(operation, backoff.WithContext(b, ctx))
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("network connectivity check failed: %w", err)
	}

	return nil
}

// cleanupAbandonedVPCs attempts to find and remove any abandoned VPCs
func (p *AWSProvider) cleanupAbandonedVPCs(ctx context.Context) error {
	l := logger.Get()
	l.Info("Attempting to cleanup abandoned VPCs")

	// List all VPCs
	result, err := p.EC2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return fmt.Errorf("failed to list VPCs: %w", err)
	}

	for _, vpc := range result.Vpcs {
		// Skip the default VPC
		if aws.ToBool(vpc.IsDefault) {
			continue
		}

		// Check if VPC has the andaime tag
		isAndaimeVPC := false
		for _, tag := range vpc.Tags {
			if aws.ToString(tag.Key) == "andaime" {
				isAndaimeVPC = true
				break
			}
		}

		if isAndaimeVPC {
			vpcID := aws.ToString(vpc.VpcId)
			l.Infof("Found abandoned VPC %s, cleaning up dependencies...", vpcID)

			// 1. Delete subnets
			subnets, err := p.EC2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: []string{vpcID},
					},
				},
			})
			if err != nil {
				l.Warnf("Failed to describe subnets for VPC %s: %v", vpcID, err)
				continue
			}

			for _, subnet := range subnets.Subnets {
				_, err := p.EC2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
					SubnetId: subnet.SubnetId,
				})
				if err != nil {
					l.Warnf("Failed to delete subnet %s: %v", aws.ToString(subnet.SubnetId), err)
				}
			}

			// 2. Delete security groups (except default)
			sgs, err := p.EC2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: []string{vpcID},
					},
				},
			})
			if err != nil {
				l.Warnf("Failed to describe security groups for VPC %s: %v", vpcID, err)
				continue
			}

			for _, sg := range sgs.SecurityGroups {
				if aws.ToString(sg.GroupName) != "default" {
					_, err := p.EC2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
						GroupId: sg.GroupId,
					})
					if err != nil {
						l.Warnf(
							"Failed to delete security group %s: %v",
							aws.ToString(sg.GroupId),
							err,
						)
					}
				}
			}

			// 3. Delete route tables (except main)
			rts, err := p.EC2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: []string{vpcID},
					},
				},
			})
			if err != nil {
				l.Warnf("Failed to describe route tables for VPC %s: %v", vpcID, err)
				continue
			}

			for _, rt := range rts.RouteTables {
				// Skip the main route table
				isMain := false
				for _, assoc := range rt.Associations {
					if aws.ToBool(assoc.Main) {
						isMain = true
						break
					}
				}
				if isMain {
					continue
				}

				// Delete route table associations first
				for _, assoc := range rt.Associations {
					if assoc.RouteTableAssociationId != nil {
						_, err := p.EC2Client.DisassociateRouteTable(
							ctx,
							&ec2.DisassociateRouteTableInput{
								AssociationId: assoc.RouteTableAssociationId,
							},
						)
						if err != nil {
							l.Warnf(
								"Failed to disassociate route table %s: %v",
								aws.ToString(rt.RouteTableId),
								err,
							)
						}
					}
				}

				_, err := p.EC2Client.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
					RouteTableId: rt.RouteTableId,
				})
				if err != nil {
					l.Warnf(
						"Failed to delete route table %s: %v",
						aws.ToString(rt.RouteTableId),
						err,
					)
				}
			}

			// 4. Detach and delete internet gateways
			igws, err := p.EC2Client.DescribeInternetGateways(
				ctx,
				&ec2.DescribeInternetGatewaysInput{
					Filters: []ec2_types.Filter{
						{
							Name:   aws.String("attachment.vpc-id"),
							Values: []string{vpcID},
						},
					},
				},
			)
			if err != nil {
				l.Warnf("Failed to describe internet gateways for VPC %s: %v", vpcID, err)
				continue
			}

			for _, igw := range igws.InternetGateways {
				// Detach first
				_, err := p.EC2Client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
					VpcId:             aws.String(vpcID),
				})
				if err != nil {
					l.Warnf(
						"Failed to detach internet gateway %s: %v",
						aws.ToString(igw.InternetGatewayId),
						err,
					)
					continue
				}

				// Then delete
				_, err = p.EC2Client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
				})
				if err != nil {
					l.Warnf(
						"Failed to delete internet gateway %s: %v",
						aws.ToString(igw.InternetGatewayId),
						err,
					)
				}
			}

			// Finally, delete the VPC
			_, err = p.EC2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
				VpcId: aws.String(vpcID),
			})
			if err != nil {
				l.Warnf("Failed to delete VPC %s: %v", vpcID, err)
				continue
			}
			l.Infof("Successfully deleted abandoned VPC %s and its dependencies", vpcID)
		}
	}

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

// WaitUntilInstanceRunning waits for an EC2 instance to reach the running state
func (p *AWSProvider) WaitUntilInstanceRunning(
	ctx context.Context,
	input *ec2.DescribeInstancesInput,
) error {
	l := logger.Get()
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 5 * time.Second
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 10 * time.Minute

	operation := func() error {
		if err := ctx.Err(); err != nil {
			return backoff.Permanent(err)
		}

		result, err := p.EC2Client.DescribeInstances(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to describe instances: %w", err)
		}

		if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
			return fmt.Errorf("no instances found")
		}

		instance := result.Reservations[0].Instances[0]
		if instance.State == nil {
			return fmt.Errorf("instance state is nil")
		}

		switch instance.State.Name {
		case ec2_types.InstanceStateNameRunning:
			return nil
		case ec2_types.InstanceStateNameTerminated, ec2_types.InstanceStateNameShuttingDown:
			return backoff.Permanent(fmt.Errorf("instance terminated or shutting down"))
		default:
			return fmt.Errorf("instance not yet running, current state: %s", instance.State.Name)
		}
	}

	err := backoff.Retry(operation, backoff.WithContext(b, ctx))
	if err != nil {
		return fmt.Errorf("timeout waiting for instance to reach running state: %w", err)
	}

	l.Info("Instance is now running")
	return nil
}

func (p *AWSProvider) SetEC2Client(client aws_interface.EC2Clienter) {
	p.EC2Client = client
}

func (p *AWSProvider) GetEC2Client() aws_interface.EC2Clienter {
	return p.EC2Client
}

func (p *AWSProvider) GetPrimaryRegion() string {
	return p.Config.Region
}

func (p *AWSProvider) GetAccountID() string {
	return p.AccountID
}

func (p *AWSProvider) GetConfig() *aws.Config {
	return p.Config
}

// GetLatestUbuntuAMI returns the latest Ubuntu 22.04 LTS AMI ID for the specified architecture
// arch should be either "x86_64" or "arm64"
func (p *AWSProvider) GetLatestUbuntuAMI(
	ctx context.Context,
	loc string,
	arch string,
) (string, error) {
	c := p.GetEC2Client()

	input := &ec2.DescribeImagesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-*-server-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{arch},
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

	result, err := c.DescribeImages(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe images: %w", err)
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf(
			"no matching Ubuntu AMI found for location: '%s' and architecture: '%s'",
			loc, arch,
		)
	}

	// Sort images by creation date (newest first)
	sort.Slice(result.Images, func(i, j int) bool {
		iTime, _ := time.Parse(time.RFC3339, *result.Images[i].CreationDate)
		jTime, _ := time.Parse(time.RFC3339, *result.Images[j].CreationDate)
		return iTime.After(jTime)
	})

	// Return the ID of the newest image
	return *result.Images[0].ImageId, nil
}

// ListDeployments returns a list of all deployments across regions with the "andaime" tag
func (p *AWSProvider) ListDeployments(ctx context.Context) ([]DeploymentInfo, error) {
	// Get list of all AWS regions
	var cfg aws.Config
	var err error
	if p.Config != nil {
		cfg = *p.Config
	} else {
		cfg, err = awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
		}
	}

	// Set the initial region if not set
	if cfg.Region == "" {
		cfg.Region = p.Config.Region
	}

	ec2Client := ec2.NewFromConfig(cfg)
	regionsOutput, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS regions: %w", err)
	}

	var deployments []DeploymentInfo
	g, ctx := errgroup.WithContext(ctx)

	// Query each region in parallel
	for _, region := range regionsOutput.Regions {
		region := region // Create new variable for goroutine
		g.Go(func() error {
			// Create regional client
			regionalCfg := cfg.Copy()
			regionalCfg.Region = *region.RegionName
			regionalClient := ec2.NewFromConfig(regionalCfg)

			// Query VPCs with andaime tag
			vpcOutput, err := regionalClient.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("tag-key"),
						Values: []string{"andaime"},
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to list VPCs in region %s: %w", *region.RegionName, err)
			}

			// Group VPCs by deployment ID (from tags)
			deploymentVPCs := make(map[string][]ec2_types.Vpc)
			for _, vpc := range vpcOutput.Vpcs {
				var deploymentID string
				for _, tag := range vpc.Tags {
					if *tag.Key == "andaime-deployment-id" {
						deploymentID = *tag.Value
						break
					}
				}
				if deploymentID != "" {
					deploymentVPCs[deploymentID] = append(deploymentVPCs[deploymentID], vpc)
				}
			}

			// For each deployment, count instances
			for deploymentID, vpcs := range deploymentVPCs {
				// Get instance count for all VPCs in this deployment
				var instanceCount int
				for _, vpc := range vpcs {
					instanceOutput, err := regionalClient.DescribeInstances(
						ctx,
						&ec2.DescribeInstancesInput{
							Filters: []ec2_types.Filter{
								{
									Name:   aws.String("vpc-id"),
									Values: []string{*vpc.VpcId},
								},
								{
									Name:   aws.String("tag-key"),
									Values: []string{"andaime"},
								},
							},
						},
					)
					if err != nil {
						return fmt.Errorf("failed to list instances in VPC %s: %w", *vpc.VpcId, err)
					}

					for _, reservation := range instanceOutput.Reservations {
						instanceCount += len(reservation.Instances)
					}
				}

				// Create deployment info
				info := DeploymentInfo{
					ID:            deploymentID,
					Region:        *region.RegionName,
					VPCCount:      len(vpcs),
					InstanceCount: instanceCount,
					Tags:          make(map[string]string),
				}

				// Add any additional andaime-related tags from the first VPC
				if len(vpcs) > 0 {
					for _, tag := range vpcs[0].Tags {
						if strings.HasPrefix(*tag.Key, "andaime-") {
							info.Tags[*tag.Key] = *tag.Value
						}
					}
				}

				deployments = append(deployments, info)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Also check local config file for any deployments
	configDeployments := viper.GetStringMap("aws.deployments")
	for id := range configDeployments {
		// Check if this deployment was already found in AWS
		found := false
		for _, d := range deployments {
			if d.ID == id {
				found = true
				break
			}
		}

		if !found {
			// Add deployment from config
			deployments = append(deployments, DeploymentInfo{
				ID:     id,
				Region: viper.GetString(fmt.Sprintf("aws.deployments.%s.region", id)),
				Tags: map[string]string{
					"andaime-source": "local-config",
				},
			})
		}
	}

	return deployments, nil
}
