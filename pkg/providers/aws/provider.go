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
	ResourcePollingInterval    = 30 * time.Second
	UpdateQueueSize            = 1000 // Increased from 100 to prevent dropping updates
	UpdatePollingInterval      = 100 * time.Millisecond
	DefaultStackTimeout        = 30 * time.Minute
	TestStackTimeout           = 30 * time.Second
	UbuntuAMIOwner             = "099720109477" // Canonical's AWS account ID
	LengthOfDeploymentIDSuffix = 8
)

type VPCState struct {
	mu          sync.RWMutex
	vpc         *models.AWSVPC
	lastUpdated time.Time
}

// RegionalVPCManager handles VPC operations across regions
type RegionalVPCManager struct {
	states     map[string]*VPCState // region -> state
	mu         sync.RWMutex
	deployment *models.Deployment
	ec2Client  aws_interface.EC2Clienter
}

// NewRegionalVPCManager creates a new VPC manager
func NewRegionalVPCManager(
	deployment *models.Deployment,
	ec2Client aws_interface.EC2Clienter,
) *RegionalVPCManager {
	return &RegionalVPCManager{
		states:     make(map[string]*VPCState),
		deployment: deployment,
		ec2Client:  ec2Client,
	}
}

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
	vpcManager      *RegionalVPCManager
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

	// Initialize a default EC2 client
	ec2Client := ec2.NewFromConfig(awsConfig)
	ec2Clienter := &LiveEC2Client{client: ec2Client}

	provider := &AWSProvider{
		AccountID:       accountID,
		Config:          &awsConfig,
		ClusterDeployer: common.NewClusterDeployer(models.DeploymentTypeAWS),
		UpdateQueue:     make(chan display.UpdateAction, UpdateQueueSize),
		STSClient:       sts.NewFromConfig(awsConfig),
		EC2Client:       ec2Clienter,
	}

	return provider, nil
}

// getOrCreateEC2Client gets or creates an EC2 client for a specific region
func (p *AWSProvider) getOrCreateEC2Client(
	ctx context.Context,
	region string,
) (aws_interface.EC2Clienter, error) {
	// If no global model, create a temporary EC2 client
	if display.GetGlobalModelFunc() == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
		}
		return &LiveEC2Client{client: ec2.NewFromConfig(cfg)}, nil
	}

	m := display.GetGlobalModelFunc()
	if m.Deployment == nil {
		m.Deployment = &models.Deployment{}
	}

	if m.Deployment.AWS == nil {
		m.Deployment.AWS = &models.AWSDeployment{
			RegionalResources: &models.RegionalResources{
				Clients: make(map[string]aws_interface.EC2Clienter),
			},
		}
	}

	if m.Deployment.AWS.RegionalResources.Clients == nil {
		m.Deployment.AWS.RegionalResources.Clients = make(map[string]aws_interface.EC2Clienter)
	}

	if client := m.Deployment.AWS.RegionalResources.GetClient(region); client != nil {
		return client, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
	}

	var ec2Client aws_interface.EC2Clienter
	p.ConfigMutex.Lock()
	if client := m.Deployment.AWS.RegionalResources.GetClient(region); client != nil {
		ec2Client = client
	} else {
		ec2Client = &LiveEC2Client{client: ec2.NewFromConfig(cfg)}
		m.Deployment.AWS.RegionalResources.Clients[region] = ec2Client
	}
	p.ConfigMutex.Unlock()

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
		region := machine.GetRegion()
		if region == "" {
			continue
		}
		// Convert zone to region if necessary
		if len(region) > 0 && region[len(region)-1] >= 'a' && region[len(region)-1] <= 'z' {
			region = region[:len(region)-1]
		}
		regions[region] = struct{}{}
	}

	return nil
}

// CreateVPCInfrastructure creates a complete VPC infrastructure in a given region
func (p *AWSProvider) CreateVPCInfrastructure(
	ctx context.Context,
	region string,
) (*models.AWSVPC, error) {
	l := logger.Get()
	l.Infof("Creating VPC infrastructure in region %s", region)

	// Create EC2 client for region
	ec2Client, err := p.getOrCreateEC2Client(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client: %w", err)
	}

	// Create VPC
	vpc, err := p.createVPC(ctx, region, ec2Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	// Create Internet Gateway and attach to VPC
	igwID, err := p.createAndAttachInternetGateway(ctx, region, vpc.VPCID, ec2Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create and attach internet gateway: %w", err)
	}
	vpc.InternetGatewayID = igwID

	// Create subnets
	err = p.createVPCSubnets(ctx, region, vpc, ec2Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnets: %w", err)
	}

	// Create and configure route table
	err = p.configureVPCRouting(ctx, region, vpc, ec2Client)
	if err != nil {
		return nil, fmt.Errorf("failed to configure routing: %w", err)
	}

	// Create security group
	sgID, err := p.createSecurityGroup(ctx, region, vpc.VPCID, ec2Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}
	vpc.SecurityGroupID = sgID

	l.Infof("Successfully created VPC infrastructure in region %s", region)
	return vpc, nil
}

// createVPC creates a new VPC with the specified CIDR block
func (p *AWSProvider) createVPC(
	ctx context.Context,
	region string,
	ec2Client aws_interface.EC2Clienter,
) (*models.AWSVPC, error) {
	vpcResult, err := ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(VPCCidrBlock),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeVpc,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("andaime-vpc-%s", region)),
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	// Enable DNS hostnames
	_, err = ec2Client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:              vpcResult.Vpc.VpcId,
		EnableDnsHostnames: &ec2_types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enable DNS hostnames: %w", err)
	}

	return &models.AWSVPC{
		VPCID: *vpcResult.Vpc.VpcId,
	}, nil
}

// createAndAttachInternetGateway creates an Internet Gateway and attaches it to the VPC
func (p *AWSProvider) createAndAttachInternetGateway(
	ctx context.Context,
	region, vpcID string,
	ec2Client aws_interface.EC2Clienter,
) (string, error) {
	// Create Internet Gateway
	igwResult, err := ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeInternetGateway,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("andaime-igw-%s", region)),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create internet gateway: %w", err)
	}

	// Attach Internet Gateway to VPC
	_, err = ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igwResult.InternetGateway.InternetGatewayId,
		VpcId:             aws.String(vpcID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach internet gateway: %w", err)
	}

	return *igwResult.InternetGateway.InternetGatewayId, nil
}

// createVPCSubnets creates the public and private subnets in the VPC
func (p *AWSProvider) createVPCSubnets(
	ctx context.Context,
	region string,
	vpc *models.AWSVPC,
	ec2Client aws_interface.EC2Clienter,
) error {
	// Get available AZs
	azResult, err := ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
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
	})
	if err != nil {
		return fmt.Errorf("failed to get availability zones: %w", err)
	}

	if len(azResult.AvailabilityZones) < MinRequiredAZs {
		return fmt.Errorf("region %s does not have enough availability zones", region)
	}

	// Create subnets
	subnets := []struct {
		cidr   string
		az     string
		name   string
		public bool
	}{
		{PublicSubnet1CIDR, *azResult.AvailabilityZones[0].ZoneName, "public-1", true},
		{PublicSubnet2CIDR, *azResult.AvailabilityZones[1].ZoneName, "public-2", true},
		{PrivateSubnet1CIDR, *azResult.AvailabilityZones[0].ZoneName, "private-1", false},
		{PrivateSubnet2CIDR, *azResult.AvailabilityZones[1].ZoneName, "private-2", false},
	}

	for _, subnet := range subnets {
		subnetResult, err := ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            aws.String(vpc.VPCID),
			CidrBlock:        aws.String(subnet.cidr),
			AvailabilityZone: aws.String(subnet.az),
			TagSpecifications: []ec2_types.TagSpecification{
				{
					ResourceType: ec2_types.ResourceTypeSubnet,
					Tags: []ec2_types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("andaime-%s-%s", subnet.name, region)),
						},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create subnet %s: %w", subnet.name, err)
		}

		if subnet.public {
			vpc.PublicSubnetIDs = append(vpc.PublicSubnetIDs, *subnetResult.Subnet.SubnetId)
		} else {
			vpc.PrivateSubnetIDs = append(vpc.PrivateSubnetIDs, *subnetResult.Subnet.SubnetId)
		}
	}

	return nil
}

// configureVPCRouting creates and configures the route table for the VPC
func (p *AWSProvider) configureVPCRouting(
	ctx context.Context,
	region string,
	vpc *models.AWSVPC,
	ec2Client aws_interface.EC2Clienter,
) error {
	// Create route table
	rtResult, err := ec2Client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpc.VPCID),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeRouteTable,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("andaime-rt-%s", region)),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create route table: %w", err)
	}

	// Add route to Internet Gateway
	_, err = ec2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         rtResult.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(vpc.InternetGatewayID),
	})
	if err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}

	// Associate public subnets with route table
	for _, subnetID := range vpc.PublicSubnetIDs {
		_, err = ec2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: rtResult.RouteTable.RouteTableId,
			SubnetId:     aws.String(subnetID),
		})
		if err != nil {
			return fmt.Errorf("failed to associate route table with subnet %s: %w", subnetID, err)
		}
	}

	vpc.PublicRouteTableID = *rtResult.RouteTable.RouteTableId
	return nil
}

// createSecurityGroup creates and configures the security group for the VPC
func (p *AWSProvider) createSecurityGroup(
	ctx context.Context,
	region, vpcID string,
	ec2Client aws_interface.EC2Clienter,
) (string, error) {
	sgResult, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("andaime-sg-%s", region)),
		Description: aws.String("Security group for Andaime instances"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeSecurityGroup,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("andaime-sg-%s", region)),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}

	// Add security group rules

	ipPermissions := []ec2_types.IpPermission{}
	for _, port := range p.vpcManager.deployment.AllowedPorts {
		ipPermissions = append(ipPermissions, ec2_types.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(validatePort(port)),
			ToPort:     aws.Int32(validatePort(port)),
			IpRanges:   []ec2_types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
		})
	}

	_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       sgResult.GroupId,
		IpPermissions: ipPermissions,
	})
	if err != nil {
		return "", fmt.Errorf("failed to authorize security group ingress: %w", err)
	}

	return *sgResult.GroupId, nil
}

// validatePort ensures port number is within valid int32 range
func validatePort(port int) int32 {
	if port < 0 || port > 65535 {
		return 0 // Or handle error appropriately
	}
	return int32(port)
}

func (p *AWSProvider) createVPCInfrastructure(
	ctx context.Context,
	region string,
) (*models.AWSVPC, error) {
	l := logger.Get()
	l.Infof("Creating VPC infrastructure in region %s", region)

	// Create EC2 client for the region
	regionalClient, err := p.getOrCreateEC2Client(ctx, region)
	if err != nil {
		l.Errorf("Failed to create EC2 client for region %s: %v", region, err)
		return nil, fmt.Errorf("failed to create EC2 client for region %s: %w", region, err)
	}

	// Generate a unique deployment ID for tracking
	deploymentID := fmt.Sprintf("andaime-%s", generateRandomString(LengthOfDeploymentIDSuffix))

	// Create VPC with CIDR block and enhanced tagging
	vpcOut, err := regionalClient.CreateVpc(ctx, &ec2.CreateVpcInput{
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
					{
						Key:   aws.String("andaime-deployment-id"),
						Value: aws.String(deploymentID),
					},
					{
						Key:   aws.String("andaime-region"),
						Value: aws.String(region),
					},
				},
			},
		},
	})
	if err != nil {
		l.Errorf("Failed to create VPC in region %s: %v", region, err)
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	l.Infof("Created VPC %s in region %s", *vpcOut.Vpc.VpcId, region)

	// Create security group with enhanced tagging
	sgOut, err := regionalClient.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
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
					{
						Key:   aws.String("andaime"),
						Value: aws.String("true"),
					},
					{
						Key:   aws.String("andaime-deployment-id"),
						Value: aws.String(deploymentID),
					},
					{
						Key:   aws.String("andaime-region"),
						Value: aws.String(region),
					},
				},
			},
		},
	})
	if err != nil {
		l.Errorf("Failed to create security group in VPC %s: %v", *vpcOut.Vpc.VpcId, err)
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}

	l.Infof("Created security group %s in VPC %s", *sgOut.GroupId, *vpcOut.Vpc.VpcId)

	// Allow inbound traffic
	_, err = regionalClient.AuthorizeSecurityGroupIngress(
		ctx,
		&ec2.AuthorizeSecurityGroupIngressInput{
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
		},
	)
	if err != nil {
		l.Errorf("Failed to authorize security group ingress for group %s: %v", *sgOut.GroupId, err)
		return nil, fmt.Errorf("failed to authorize security group ingress: %w", err)
	}

	l.Infof("Authorized security group ingress for %s", *sgOut.GroupId)

	// Create VPC object
	vpc := &models.AWSVPC{
		VPCID:           *vpcOut.Vpc.VpcId,
		SecurityGroupID: *sgOut.GroupId,
	}

	// Initialize VPC manager if not already done
	if p.vpcManager == nil {
		m := display.GetGlobalModelFunc()
		if m == nil || m.Deployment == nil {
			l.Error("Global model or deployment is nil")
			return nil, fmt.Errorf("global model or deployment is nil")
		}
		p.vpcManager = NewRegionalVPCManager(m.Deployment, p.EC2Client)
	}

	// Save VPC to regional resources
	err = p.vpcManager.UpdateVPC(region, func(existingVPC *models.AWSVPC) error {
		*existingVPC = *vpc
		return nil
	})
	if err != nil {
		l.Errorf("Failed to save VPC %s to regional resources: %v", vpc.VPCID, err)
		return nil, fmt.Errorf("failed to save VPC to regional resources: %w", err)
	}

	l.Infof("Successfully created and saved VPC %s in region %s", vpc.VPCID, region)

	return vpc, nil
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
func (p *AWSProvider) CreateVpc(ctx context.Context, region string) error {
	l := logger.Get()
	l.Debugf("Creating VPC in region %s", region)

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// Initialize VPC manager if needed
	if p.vpcManager == nil {
		p.vpcManager = NewRegionalVPCManager(m.Deployment, p.EC2Client)
	}

	// Get or create EC2 client for this region
	regionalClient, err := p.getOrCreateEC2Client(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to get EC2 client: %w", err)
	}

	vpc, err := p.createVPCInfrastructure(ctx, region)
	if err != nil {
		return err
	}

	// Update display state to pending
	p.updateVPCDisplayState(m, models.ResourceStatePending)

	// Enable DNS hostnames
	modifyVpcAttributeInput := &ec2.ModifyVpcAttributeInput{
		VpcId:              aws.String(vpc.VPCID),
		EnableDnsHostnames: &ec2_types.AttributeBooleanValue{Value: aws.Bool(true)},
	}

	_, err = regionalClient.ModifyVpcAttribute(ctx, modifyVpcAttributeInput)
	if err != nil {
		return fmt.Errorf("failed to enable DNS hostnames: %w", err)
	}

	// Update display state to running
	p.updateVPCDisplayState(m, models.ResourceStateRunning)

	// Save VPC configuration
	if err := p.vpcManager.SaveVPCConfig(region); err != nil {
		l.Warnf("Failed to save VPC configuration: %v", err)
	}

	l.Debugf("Regional VPC (Region: %s) created successfully with ID %s",
		region, vpc.VPCID)

	return nil
}

func (p *AWSProvider) CreateInfrastructure(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// First, validate all regions have sufficient AZs before doing anything else
	l.Info("Pre-validating availability zones for all regions")
	regionsToValidate := make(map[string]bool)

	// Collect unique regions and validate them first
	for _, machine := range m.Deployment.GetMachines() {
		region := machine.GetRegion()
		// Convert zone to region if necessary
		if len(region) > 0 && region[len(region)-1] >= 'a' && region[len(region)-1] <= 'z' {
			region = region[:len(region)-1]
		}
		regionsToValidate[region] = true
	}

	// Validate all regions upfront
	for region := range regionsToValidate {
		l.Debug(fmt.Sprintf("Pre-validating region: %s", region))
		if err := p.validateRegionZones(ctx, region); err != nil {
			l.Error(fmt.Sprintf("Pre-validation failed for region %s: %v", region, err))

			// Update display for the first machine in this region
			for _, machine := range m.Deployment.GetMachines() {
				machineRegion := machine.GetRegion()
				if len(machineRegion) > 0 && machineRegion[len(machineRegion)-1] >= 'a' &&
					machineRegion[len(machineRegion)-1] <= 'z' {
					machineRegion = machineRegion[:len(machineRegion)-1]
				}
				if machineRegion == region {
					m.QueueUpdate(display.UpdateAction{
						MachineName: machine.GetName(),
						UpdateData: display.UpdateData{
							UpdateType:    display.UpdateTypeResource,
							ResourceType:  display.ResourceType("Infrastructure"),
							ResourceState: models.ResourceStateFailed,
						},
					})
					break
				}
			}

			// Force immediate program termination
			if prog := display.GetGlobalProgramFunc(); prog != nil {
				prog.Quit()
			}

			return fmt.Errorf("region %s validation failed: %w", region, err)
		}
	}

	// Only proceed with infrastructure creation if all regions are validated
	l.Info("All regions validated successfully, proceeding with infrastructure creation")

	return nil
}

// Create infrastructure in all regions concurrently
func (p *AWSProvider) CreateRegionalResources(
	ctx context.Context,
	regions []string,
) error {
	l := logger.Get()
	l.Infof("Creating regional resources for regions: %v", regions)

	var eg errgroup.Group
	var mu sync.Mutex
	var errors []error

	uniqueRegions := make(map[string]bool)
	for _, region := range regions {
		uniqueRegions[region] = true
	}

	for region := range uniqueRegions {
		region := region // capture for goroutine
		eg.Go(func() error {
			l.Debugf("Setting up infrastructure for region: %s", region)
			err := p.setupRegionalInfrastructure(ctx, region)
			if err != nil {
				l.Errorf("Failed to setup infrastructure for region %s: %v", region, err)
				mu.Lock()
				errors = append(errors, fmt.Errorf("region %s: %w", region, err))
				mu.Unlock()
				return err
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		l.Errorf("Error creating regional resources: %v", err)

		// Combine all errors
		var combinedErr error
		mu.Lock()
		for _, e := range errors {
			if combinedErr == nil {
				combinedErr = e
			} else {
				combinedErr = fmt.Errorf("%v; %w", combinedErr, e)
			}
		}
		mu.Unlock()

		return fmt.Errorf("failed to deploy VMs in parallel: %w", combinedErr)
	}

	return nil
}

// Setup infrastructure for a single region
func (p *AWSProvider) setupRegionalInfrastructure(ctx context.Context, region string) error {
	l := logger.Get()
	l.Debugf("Setting up infrastructure for region: %s", region)

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// Ensure regional resources are initialized
	if m.Deployment.AWS.RegionalResources == nil {
		m.Deployment.AWS.RegionalResources = &models.RegionalResources{
			VPCs:    make(map[string]*models.AWSVPC),
			Clients: make(map[string]aws_interface.EC2Clienter),
		}
	}

	// Debug current state
	l.Debugf("Current deployment state: %+v", m.Deployment.AWS)
	if m.Deployment.AWS.RegionalResources != nil {
		l.Debugf("Current VPCs: %+v", m.Deployment.AWS.RegionalResources.VPCs)
	}

	// Initialize VPC manager if needed
	if p.vpcManager == nil {
		l.Debug("Initializing VPC manager")
		p.vpcManager = NewRegionalVPCManager(m.Deployment, p.EC2Client)
	}

	// Step 1: Get availability zones
	azs, err := p.getRegionAvailabilityZones(ctx, region)
	if err != nil {
		l.Errorf("Failed to get availability zones for region %s: %v", region, err)
		return fmt.Errorf("failed to get availability zones for region %s: %w", region, err)
	}
	l.Debugf("Found %d availability zones in region %s", len(azs), region)

	// Step 2: Create VPC and security groups
	l.Debugf("Creating VPC in region %s", region)
	if err := p.createVPCWithRetry(ctx, region); err != nil {
		l.Errorf("Failed to create VPC in region %s: %v", region, err)
		return fmt.Errorf("failed to create VPC in region %s: %w", region, err)
	}

	// Get VPC state
	state := p.vpcManager.GetOrCreateVPCState(region)
	state.mu.RLock()
	vpc := state.vpc
	state.mu.RUnlock()

	l.Debugf("VPC state after creation for region %s: %+v", region, vpc)

	if vpc == nil {
		l.Error("VPC is nil after creation!")
		// Attempt to retrieve VPC from regional resources
		vpc = m.Deployment.AWS.RegionalResources.GetVPC(region)
		if vpc == nil {
			l.Errorf("Could not find VPC in regional resources for region %s", region)
			return fmt.Errorf("VPC not found for region %s", region)
		}
		l.Infof("Retrieved VPC %s from regional resources", vpc.VPCID)
	}

	// Step 4: Create subnets
	if err := p.createRegionalSubnets(ctx, region, azs[:2]); err != nil {
		l.Errorf("Failed to create subnets in region %s: %v", region, err)
		return fmt.Errorf("failed to create subnets in region %s: %w", region, err)
	}

	// Step 5: Setup networking components
	if err := p.setupNetworking(ctx, region); err != nil {
		l.Errorf("Failed to setup networking in region %s: %v", region, err)
		return fmt.Errorf("failed to setup networking in region %s: %w", region, err)
	}

	// Step 6: Save and update display
	if err := p.saveInfrastructureToConfig(); err != nil {
		l.Errorf("Failed to save infrastructure config for region %s: %v", region, err)
		return fmt.Errorf(
			"failed to save infrastructure IDs to config for region %s: %w",
			region,
			err,
		)
	}

	p.updateInfrastructureDisplay(models.ResourceStateSucceeded)

	l.Infof("AWS infrastructure created successfully for region: %s", region)
	return nil
}

// Get availability zones for a region
func (p *AWSProvider) getRegionAvailabilityZones(
	ctx context.Context,
	region string,
) ([]ec2_types.AvailabilityZone, error) {
	// Create a regional EC2 client
	regionalCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
	}
	regionalClient := ec2.NewFromConfig(regionalCfg)

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

	azOutput, err := regionalClient.DescribeAvailabilityZones(ctx, azInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability zones for region %s: %w", region, err)
	}

	if len(azOutput.AvailabilityZones) < MinRequiredAZs {
		return nil, fmt.Errorf("region %s does not have at least 2 availability zones", region)
	}

	return azOutput.AvailabilityZones, nil
}

// Create subnets in a region
func (p *AWSProvider) createRegionalSubnets(
	ctx context.Context,
	region string,
	azs []ec2_types.AvailabilityZone,
) error {
	l := logger.Get()
	l.Debugf("Creating regional subnets for region %s", region)

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil || m.Deployment.AWS == nil {
		return fmt.Errorf("deployment model not properly initialized")
	}

	// Initialize VPC manager if needed
	if p.vpcManager == nil {
		p.vpcManager = NewRegionalVPCManager(m.Deployment, p.EC2Client)
	}

	state := p.vpcManager.GetOrCreateVPCState(region)
	if state == nil {
		return fmt.Errorf("failed to get VPC state for region %s", region)
	}

	state.mu.RLock()
	vpc := state.vpc
	state.mu.RUnlock()

	if vpc == nil {
		return fmt.Errorf("VPC not found for region %s", region)
	}

	l.Debugf("Creating subnets in VPC %s", vpc.VPCID)

	return p.vpcManager.UpdateVPC(region, func(vpc *models.AWSVPC) error {
		for i, az := range azs {
			// Create public subnet
			l.Debugf("Creating public subnet in AZ %s", *az.ZoneName)
			publicSubnet, err := p.createSubnet(
				ctx,
				region,
				vpc.VPCID,
				*az.ZoneName,
				i*2, //nolint:mnd
				"public",
			)
			if err != nil {
				return fmt.Errorf("failed to create public subnet: %w", err)
			}
			vpc.PublicSubnetIDs = append(vpc.PublicSubnetIDs, *publicSubnet.Subnet.SubnetId)

			// Create private subnet
			l.Debugf("Creating private subnet in AZ %s", *az.ZoneName)
			privateSubnet, err := p.createSubnet(
				ctx,
				region,
				vpc.VPCID,
				*az.ZoneName,
				i*2+1,
				"private",
			)
			if err != nil {
				return fmt.Errorf("failed to create private subnet: %w", err)
			}
			vpc.PrivateSubnetIDs = append(vpc.PrivateSubnetIDs, *privateSubnet.Subnet.SubnetId)
		}
		return nil
	})
}

// Create a single subnet
func (p *AWSProvider) createSubnet(
	ctx context.Context,
	region string,
	vpcID string,
	azName string,
	index int,
	subnetType string,
) (*ec2.CreateSubnetOutput, error) {
	input := &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		AvailabilityZone: aws.String(azName),
		CidrBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", index)),
		TagSpecifications: []ec2_types.TagSpecification{
			{
				ResourceType: ec2_types.ResourceTypeSubnet,
				Tags: []ec2_types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("%s-subnet-%s", subnetType, azName)),
					},
					{Key: aws.String("Type"), Value: aws.String(subnetType)},
				},
			},
		},
	}

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return nil, fmt.Errorf("global model or deployment is nil")
	}
	regionalClient := m.Deployment.AWS.RegionalResources.Clients[region]

	subnet, err := regionalClient.CreateSubnet(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s subnet in %s: %w", subnetType, azName, err)
	}

	return subnet, nil
}

// Setup networking components (IGW, route tables, etc.)
func (p *AWSProvider) setupNetworking(ctx context.Context, region string) error {
	l := logger.Get()
	l.Debugf("Setting up networking for region %s", region)

	if p.vpcManager == nil {
		return fmt.Errorf("VPC manager not initialized")
	}

	// Create a context with a timeout to prevent indefinite waiting
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	// Get all VPCs for this region
	vpcStates := p.vpcManager.GetAllVPCStates(region)
	if len(vpcStates) == 0 {
		l.Warn("No VPC states found for region")
		return fmt.Errorf("no VPC states found for region %s", region)
	}

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	regionalClient := m.Deployment.AWS.RegionalResources.Clients[region]

	// Use errgroup to parallelize VPC networking setup with timeout
	eg, ctx := errgroup.WithContext(ctx)

	for _, state := range vpcStates {
		state := state // capture loop variable
		eg.Go(func() error {
			state.mu.RLock()
			vpc := state.vpc
			state.mu.RUnlock()

			if vpc == nil {
				l.Warn("Skipping nil VPC in networking setup")
				return nil
			}

			l.Debugf("Setting up networking for VPC %s", vpc.VPCID)

			// Verify VPC exists and is in a valid state
			vpcDesc, err := regionalClient.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
				VpcIds: []string{vpc.VPCID},
			})
			if err != nil {
				l.Errorf("Failed to describe VPC %s: %v", vpc.VPCID, err)
				return fmt.Errorf("failed to describe VPC %s: %w", vpc.VPCID, err)
			}
			if len(vpcDesc.Vpcs) == 0 {
				l.Errorf("VPC %s not found", vpc.VPCID)
				return fmt.Errorf("VPC %s not found", vpc.VPCID)
			}

			// Create Internet Gateway with retry
			var igw *ec2.CreateInternetGatewayOutput
			err = backoff.Retry(func() error {
				var err error
				igw, err = regionalClient.CreateInternetGateway(
					ctx,
					&ec2.CreateInternetGatewayInput{
						TagSpecifications: []ec2_types.TagSpecification{
							{
								ResourceType: ec2_types.ResourceTypeInternetGateway,
								Tags: []ec2_types.Tag{
									{
										Key:   aws.String("Name"),
										Value: aws.String(fmt.Sprintf("andaime-igw-%s", vpc.VPCID)),
									},
									{
										Key:   aws.String("VPC"),
										Value: aws.String(vpc.VPCID),
									},
								},
							},
						},
					},
				)
				return err
			}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
			if err != nil {
				l.Errorf("Failed to create internet gateway for VPC %s: %v", vpc.VPCID, err)
				return fmt.Errorf(
					"failed to create internet gateway for VPC %s: %w",
					vpc.VPCID,
					err,
				)
			}

			// Attach Internet Gateway with retry
			err = backoff.Retry(func() error {
				_, err := regionalClient.AttachInternetGateway(
					ctx,
					&ec2.AttachInternetGatewayInput{
						InternetGatewayId: aws.String(*igw.InternetGateway.InternetGatewayId),
						VpcId:             aws.String(vpc.VPCID),
					},
				)
				return err
			}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
			if err != nil {
				l.Errorf("Failed to attach internet gateway to VPC %s: %v", vpc.VPCID, err)
				return fmt.Errorf("failed to attach internet gateway to VPC %s: %w", vpc.VPCID, err)
			}

			// Update VPC state with Internet Gateway ID
			if err := p.vpcManager.UpdateVPC(region, func(v *models.AWSVPC) error {
				if v.VPCID == vpc.VPCID {
					v.InternetGatewayID = *igw.InternetGateway.InternetGatewayId
				}
				return nil
			}); err != nil {
				l.Errorf("Failed to update VPC state for VPC %s: %v", vpc.VPCID, err)
				return fmt.Errorf("failed to update VPC state: %w", err)
			}

			// Setup routing for this VPC
			if err := p.setupRouting(ctx, region, vpc); err != nil {
				l.Errorf("Failed to setup routing for VPC %s: %v", vpc.VPCID, err)
				return fmt.Errorf("failed to setup routing for VPC %s: %w", vpc.VPCID, err)
			}

			// Verify Internet Gateway and Route Table
			igws, err := regionalClient.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("attachment.vpc-id"),
						Values: []string{vpc.VPCID},
					},
				},
			})
			if err != nil {
				l.Errorf("Failed to describe internet gateways for VPC %s: %v", vpc.VPCID, err)
			} else {
				l.Debugf("Internet Gateways for VPC %s: %d", vpc.VPCID, len(igws.InternetGateways))
				for _, gateway := range igws.InternetGateways {
					l.Debugf("IGW ID: %s", aws.ToString(gateway.InternetGatewayId))
				}
			}

			rts, err := regionalClient.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
				Filters: []ec2_types.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: []string{vpc.VPCID},
					},
				},
			})
			if err != nil {
				l.Errorf("Failed to describe route tables for VPC %s: %v", vpc.VPCID, err)
			} else {
				l.Debugf("Route tables for VPC %s: %d", vpc.VPCID, len(rts.RouteTables))
				for i, rt := range rts.RouteTables {
					l.Debugf("Route table %d ID: %s", i+1, aws.ToString(rt.RouteTableId))
					for j, route := range rt.Routes {
						l.Debugf("Route %d: Destination %s, Gateway %s", 
							j+1, 
							aws.ToString(route.DestinationCidrBlock), 
							aws.ToString(route.GatewayId),
						)
					}
				}
			}

			l.Debugf("Successfully set up networking for VPC %s", vpc.VPCID)
			return nil
		})
	}

	// Wait for all VPC networking setups to complete
	if err := eg.Wait(); err != nil {
		l.Errorf("Error setting up networking in region %s: %v", region, err)
		return fmt.Errorf("failed to set up networking in region %s: %w", region, err)
	}

	return nil
}

// Update the infrastructure display state
func (p *AWSProvider) updateInfrastructureDisplay(state models.MachineResourceState) {
	m := display.GetGlobalModelFunc()
	if m != nil && m.Deployment != nil {
		for _, machine := range m.Deployment.GetMachines() {
			m.QueueUpdate(display.UpdateAction{
				MachineName: machine.GetName(),
				UpdateData: display.UpdateData{
					UpdateType:    display.UpdateTypeResource,
					ResourceType:  "Infrastructure",
					ResourceState: state,
				},
			})
		}
	}
}

// createVPCWithRetry handles VPC creation with cleanup retry logic
func (p *AWSProvider) createVPCWithRetry(ctx context.Context, region string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// Initialize VPC manager if needed
	if p.vpcManager == nil {
		p.vpcManager = NewRegionalVPCManager(m.Deployment, p.EC2Client)
	}

	err := p.CreateVpc(ctx, region)
	if err != nil {
		if strings.Contains(err.Error(), "VpcLimitExceeded") {
			l.Info("VPC limit exceeded, attempting to clean up abandoned VPCs...")

			// Try to find and clean up any abandoned VPCs
			if cleanupErr := p.cleanupAbandonedVPCs(ctx, region); cleanupErr != nil {
				return fmt.Errorf("failed to cleanup VPCs: %w", cleanupErr)
			}

			// Retry VPC creation after cleanup
			l.Info("Retrying VPC creation after cleanup...")
			if retryErr := p.CreateVpc(ctx, region); retryErr != nil {
				return fmt.Errorf("failed to create VPC after cleanup: %w", retryErr)
			}

			l.Info("Successfully created VPC after cleanup")
		} else {
			return fmt.Errorf("failed to create VPC: %w", err)
		}
	}

	return nil
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

					// Log route table associations
					l.Debugf("Route table associations for %s:", *rt.RouteTableId)
					for _, assoc := range rt.Associations {
						l.Debugf("- Association: SubnetID: %s, Main: %v",
							aws.ToString(assoc.SubnetId),
							aws.ToBool(assoc.Main))
					}

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

					// Additional diagnostic information
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
						l.Errorf("Failed to describe internet gateways: %v", err)
					} else {
						l.Debugf("Internet Gateways for VPC %s: %d", vpcID, len(igws.InternetGateways))
						for _, igw := range igws.InternetGateways {
							l.Debugf("IGW ID: %s", aws.ToString(igw.InternetGatewayId))
						}
					}

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
func (p *AWSProvider) cleanupAbandonedVPCs(ctx context.Context, region string) error {
	l := logger.Get()
	l.Info("Attempting to cleanup abandoned VPCs")

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	regionalClient := m.Deployment.AWS.RegionalResources.Clients[region]

	// List all VPCs
	result, err := regionalClient.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
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
			subnets, err := regionalClient.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
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
				_, err := regionalClient.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
					SubnetId: subnet.SubnetId,
				})
				if err != nil {
					l.Warnf("Failed to delete subnet %s: %v", aws.ToString(subnet.SubnetId), err)
				}
			}

			// 2. Delete security groups (except default)
			sgs, err := regionalClient.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
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
				if aws.ToString(sg.GroupName) != DefaultName {
					_, err := regionalClient.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
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
			rts, err := regionalClient.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
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
						_, err := regionalClient.DisassociateRouteTable(
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

				_, err := regionalClient.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
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
			igws, err := regionalClient.DescribeInternetGateways(
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
				_, err := regionalClient.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
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
				_, err = regionalClient.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
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
			_, err = regionalClient.DeleteVpc(ctx, &ec2.DeleteVpcInput{
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

// Helper functions for VPC routing setup
func (p *AWSProvider) setupRouting(ctx context.Context, region string, vpc *models.AWSVPC) error {
	l := logger.Get()
	// Create public route table
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	regionalClient := m.Deployment.AWS.RegionalResources.Clients[region]

	l.Debug("Creating public route table")
	publicRT, err := regionalClient.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpc.VPCID),
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
	vpc.PublicRouteTableID = *publicRT.RouteTable.RouteTableId

	// Create route to Internet Gateway
	if err := p.createInternetGatewayRoute(ctx, region, vpc); err != nil {
		return err
	}

	// Associate public subnets with route table
	return p.associatePublicSubnets(ctx, region, vpc)
}

func (p *AWSProvider) createInternetGatewayRoute(
	ctx context.Context,
	region string,
	vpc *models.AWSVPC,
) error {
	l := logger.Get()
	l.Debugf("Creating internet gateway route for VPC %s", vpc.VPCID)
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	regionalClient := m.Deployment.AWS.RegionalResources.Clients[region]

	_, err := regionalClient.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(vpc.PublicRouteTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(vpc.InternetGatewayID),
	})
	return err
}

func (p *AWSProvider) associatePublicSubnets(
	ctx context.Context,
	region string,
	vpc *models.AWSVPC,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	regionalClient := m.Deployment.AWS.RegionalResources.Clients[region]

	for _, subnetID := range vpc.PublicSubnetIDs {
		l.Debugf("Associating subnet %s with route table %s", subnetID, vpc.PublicRouteTableID)
		_, err := regionalClient.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(vpc.PublicRouteTableID),
			SubnetId:     aws.String(subnetID),
		})
		if err != nil {
			return fmt.Errorf(
				"failed to associate public subnet %s with route table: %w",
				subnetID,
				err,
			)
		}
	}
	return nil
}

// VPC state display update helper
func (p *AWSProvider) updateVPCDisplayState(
	m *display.DisplayModel,
	state models.MachineResourceState,
) {
	if m != nil && m.Deployment != nil {
		for _, machine := range m.Deployment.GetMachines() {
			m.QueueUpdate(display.UpdateAction{
				MachineName: machine.GetName(),
				UpdateData: display.UpdateData{
					UpdateType:    display.UpdateTypeResource,
					ResourceType:  "VPC",
					ResourceState: state,
				},
			})
		}
	}
}

// Additional VPC manager methods
func (p *AWSProvider) GetVPCState(region string) (*VPCState, error) {
	if p.vpcManager == nil {
		return nil, fmt.Errorf("VPC manager not initialized")
	}
	return p.vpcManager.GetOrCreateVPCState(region), nil
}

func (p *AWSProvider) ValidateVPCState(region string) error {
	state, err := p.GetVPCState(region)
	if err != nil {
		return err
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.vpc == nil || state.vpc.VPCID == "" {
		return fmt.Errorf("no valid VPC found for region %s", region)
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
	if c == nil {
		return "", fmt.Errorf("EC2 client not initialized")
	}

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

// GetOrCreateVPCState gets or creates a VPC state for a region
func (rm *RegionalVPCManager) GetOrCreateVPCState(region string) *VPCState {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if state, exists := rm.states[region]; exists {
		return state
	}

	state := &VPCState{
		vpc:         rm.deployment.AWS.RegionalResources.GetVPC(region),
		lastUpdated: time.Now(),
	}
	rm.states[region] = state
	return state
}

// UpdateVPC atomically updates a VPC's state
func (rm *RegionalVPCManager) UpdateVPC(region string, updateFn func(*models.AWSVPC) error) error {
	state := rm.GetOrCreateVPCState(region)
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.vpc == nil {
		state.vpc = &models.AWSVPC{}
		rm.deployment.AWS.RegionalResources.SetVPC(region, state.vpc)
	}

	if err := updateFn(state.vpc); err != nil {
		return fmt.Errorf("failed to update VPC state: %w", err)
	}

	state.lastUpdated = time.Now()
	return nil
}

// SaveVPCConfig saves VPC configuration to viper
func (rm *RegionalVPCManager) SaveVPCConfig(region string) error {
	state := rm.GetOrCreateVPCState(region)
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.vpc == nil {
		return fmt.Errorf("no VPC found for region %s", region)
	}

	deploymentPath := fmt.Sprintf("deployments.%s", rm.deployment.UniqueID)
	viper.Set(fmt.Sprintf("%s.aws.regions.%s.vpc_id", deploymentPath, region), state.vpc.VPCID)

	return viper.WriteConfig()
}

// GetAllVPCStates returns all VPC states for a given region
func (rm *RegionalVPCManager) GetAllVPCStates(region string) []*VPCState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var states []*VPCState
	for r, state := range rm.states {
		if r == region {
			states = append(states, state)
		}
	}

	return states
}
