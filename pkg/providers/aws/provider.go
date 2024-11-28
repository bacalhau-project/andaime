package awsprovider

import (
	"context"
	"fmt"
	"sort"
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
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/viper"
)

const (
	ResourcePollingInterval = 10 * time.Second
	UpdateQueueSize         = 1000 // Increased from 100 to prevent dropping updates
	DefaultStackTimeout     = 30 * time.Minute
	TestStackTimeout        = 30 * time.Second
	UbuntuAMIOwner          = "099720109477" // Canonical's AWS account ID
)

type AWSProvider struct {
	AccountID          string
	Config             *aws.Config
	Region             string
	ClusterDeployer    common_interface.ClusterDeployerer
	UpdateQueue        chan display.UpdateAction
	VPCID              string
	InternetGatewayID  string
	PublicRouteTableID string
	PublicSubnetIDs    []string
	PrivateSubnetIDs   []string
	SecurityGroupID    string
	EC2Client          aws_interface.EC2Clienter
}

var NewAWSProviderFunc = NewAWSProvider

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
		resourceTicker := time.NewTicker(ResourcePollingInterval)
		updateTicker := time.NewTicker(100 * time.Millisecond) // Process updates more frequently
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
				if instance.NetworkInterfaces != nil && len(instance.NetworkInterfaces) > 0 {
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
				if instance.BlockDeviceMappings != nil && len(instance.BlockDeviceMappings) > 0 {
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

// CreateVpc creates a new VPC with the specified CIDR block
func (p *AWSProvider) CreateVpc(ctx context.Context) error {
	l := logger.Get()
	l.Debug("Starting VPC creation...")

	// Update all machines to show VPC creation in progress
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

	// Create the VPC
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

	createVpcOutput, err := p.EC2Client.CreateVpc(ctx, createVpcInput)
	if err != nil {
		l.Debugf("VPC creation failed: %v", err)
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	l.Debugf("VPC created successfully with ID %s", *createVpcOutput.Vpc.VpcId)
	p.VPCID = *createVpcOutput.Vpc.VpcId

	// Immediately save VPC ID to config
	if model := display.GetGlobalModelFunc(); model != nil && model.Deployment != nil {
		deploymentPath := fmt.Sprintf("deployments.%s", model.Deployment.UniqueID)
		viper.Set(fmt.Sprintf("%s.aws.vpc_id", deploymentPath), p.VPCID)
		if err := viper.WriteConfig(); err != nil {
			l.Warnf("Failed to save VPC ID to config: %v", err)
		} else {
			l.Debugf("Saved VPC ID %s to config at %s.aws.vpc_id", p.VPCID, deploymentPath)
		}
	}

	l.Debugf("VPC created successfully with ID %s", p.VPCID)

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
	if m != nil && m.Deployment != nil {
		deploymentPath := fmt.Sprintf("deployments.%s.aws", m.Deployment.UniqueID)
		viper.Set(fmt.Sprintf("%s.vpc_id", deploymentPath), p.VPCID)
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to save VPC ID to config: %w", err)
		}
		l.Infof("Saved VPC ID %s to config at %s.vpc_id", p.VPCID, deploymentPath)
	}

	return nil
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
//
// CreateInfrastructure creates the necessary AWS infrastructure including VPC, subnets,
// internet gateway, and routing tables. This is the main entry point for setting up
// the AWS networking infrastructure required for the deployment.
//
// The function performs the following steps:
// 1. Creates a VPC with CIDR block 10.0.0.0/16
// 2. Creates public and private subnets across availability zones
// 3. Sets up an internet gateway for public internet access
// 4. Configures routing tables for public and private subnets
// 5. Saves infrastructure IDs to configuration
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//
// Returns:
//   - error: Returns an error if any step of the infrastructure creation fails
func (p *AWSProvider) CreateInfrastructure(ctx context.Context) error {
	l := logger.Get()
	l.Info("Creating AWS infrastructure...")

	// Create VPC with retry on limits exceeded
	if err := p.createVPCWithRetry(ctx); err != nil {
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	// Get available availability zones
	l.Info("Getting availability zones...")
	zones, err := p.EC2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get availability zones: %w", err)
	}

	l.Info("Create Security Groups...")
	if err := p.createSecurityGroup(ctx); err != nil {
		return fmt.Errorf("failed to create security groups: %w", err)
	}

	// Create subnets across availability zones
	l.Info("Creating subnets...")
	for i, az := range zones.AvailabilityZones {
		// Create public subnet
		publicSubnetInput := &ec2.CreateSubnetInput{
			VpcId:            aws.String(p.VPCID),
			AvailabilityZone: az.ZoneName,
			CidrBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", i*2)),
			TagSpecifications: []ec2_types.TagSpecification{
				{
					ResourceType: ec2_types.ResourceTypeSubnet,
					Tags: []ec2_types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("public-subnet-%s", *az.ZoneName)),
						},
						{Key: aws.String("Type"), Value: aws.String("public")},
					},
				},
			},
		}
		publicSubnet, err := p.EC2Client.CreateSubnet(ctx, publicSubnetInput)
		if err != nil {
			return fmt.Errorf("failed to create public subnet in %s: %w", *az.ZoneName, err)
		}
		p.PublicSubnetIDs = append(p.PublicSubnetIDs, *publicSubnet.Subnet.SubnetId)

		// Create private subnet
		privateSubnetInput := &ec2.CreateSubnetInput{
			VpcId:            aws.String(p.VPCID),
			AvailabilityZone: az.ZoneName,
			CidrBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", i*2+1)),
			TagSpecifications: []ec2_types.TagSpecification{
				{
					ResourceType: ec2_types.ResourceTypeSubnet,
					Tags: []ec2_types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("private-subnet-%s", *az.ZoneName)),
						},
						{Key: aws.String("Type"), Value: aws.String("private")},
					},
				},
			},
		}
		privateSubnet, err := p.EC2Client.CreateSubnet(ctx, privateSubnetInput)
		if err != nil {
			return fmt.Errorf("failed to create private subnet in %s: %w", *az.ZoneName, err)
		}
		p.PrivateSubnetIDs = append(p.PrivateSubnetIDs, *privateSubnet.Subnet.SubnetId)
	}

	// Create Internet Gateway
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
	p.InternetGatewayID = *igw.InternetGateway.InternetGatewayId

	// Attach Internet Gateway to VPC
	l.Info("Attaching Internet Gateway to VPC...")
	_, err = p.EC2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(p.InternetGatewayID),
		VpcId:             aws.String(p.VPCID),
	})
	if err != nil {
		return fmt.Errorf("failed to attach internet gateway: %w", err)
	}

	// Create public route table
	l.Info("Creating route tables...")
	publicRT, err := p.EC2Client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(p.VPCID),
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
	p.PublicRouteTableID = *publicRT.RouteTable.RouteTableId

	// Create route to Internet Gateway
	_, err = p.EC2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(p.PublicRouteTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(p.InternetGatewayID),
	})
	if err != nil {
		return fmt.Errorf("failed to create route to internet gateway: %w", err)
	}

	// Associate public subnets with public route table
	for _, subnetID := range p.PublicSubnetIDs {
		_, err = p.EC2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(p.PublicRouteTableID),
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

	// Save infrastructure IDs to config
	if err := p.saveInfrastructureToConfig(); err != nil {
		return fmt.Errorf("failed to save infrastructure IDs to config: %w", err)
	}

	l.Info("Infrastructure created successfully")
	return nil
}

// createVPCWithRetry handles VPC creation with cleanup retry logic
func (p *AWSProvider) createVPCWithRetry(ctx context.Context) error {
	if err := p.CreateVpc(ctx); err != nil {
		if strings.Contains(err.Error(), "VpcLimitExceeded") {
			// Try to find and clean up any abandoned VPCs
			if cleanupErr := p.cleanupAbandonedVPCs(ctx); cleanupErr != nil {
				return fmt.Errorf("failed to cleanup VPCs: %w", cleanupErr)
			}
			// Retry VPC creation after cleanup
			if retryErr := p.CreateVpc(ctx); retryErr != nil {
				return fmt.Errorf("failed to create VPC after cleanup: %w", retryErr)
			}
		} else {
			return err
		}
	}

	// Wait for VPC to be available
	logger.Get().Info("Waiting for VPC to be available...")
	return p.waitForVPCAvailable(ctx)
}

func (p *AWSProvider) createSecurityGroup(ctx context.Context) error {
	l := logger.Get()
	l.Info("Creating security groups...")
	sgOutput, err := p.GetEC2Client().CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName: aws.String("andaime-sg"),
		VpcId:     aws.String(p.VPCID),
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
		return fmt.Errorf("failed to create security group: %w", err)
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
		return fmt.Errorf("failed to authorize security group ingress: %w", err)
	}
	p.SecurityGroupID = *sgOutput.GroupId

	return nil
}

// saveInfrastructureToConfig saves all infrastructure IDs to the configuration
func (p *AWSProvider) saveInfrastructureToConfig() error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	basePath := m.Deployment.ViperPath
	toSave := map[string]interface{}{
		"vpc_id":              p.VPCID,
		"public_subnet_ids":   p.PublicSubnetIDs,
		"private_subnet_ids":  p.PrivateSubnetIDs,
		"internet_gateway_id": p.InternetGatewayID,
		"route_table_id":      p.PublicRouteTableID,
	}

	for key, value := range toSave {
		viper.Set(fmt.Sprintf("%s.%s", basePath, key), value)
	}

	return viper.WriteConfig()
}

func (p *AWSProvider) WaitForNetworkConnectivity(ctx context.Context) error {
	l := logger.Get()
	l.Debugf("Starting network connectivity check with VPC ID %s", p.VPCID)

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 5 * time.Second
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 5 * time.Minute

	l.Info("Starting network connectivity check...")
	l.Infof("Using VPC ID: %s", p.VPCID)

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
					Values: []string{p.VPCID},
				},
			},
		}

		l.Info("Attempting to describe route tables for VPC connectivity check...")
		l.Debugf("Looking for route tables in VPC %s with internet gateway routes", p.VPCID)
		result, err := p.EC2Client.DescribeRouteTables(ctx, input)
		if err != nil {
			l.Debugf("Failed to describe route tables: %v", err)
			return fmt.Errorf("failed to describe route tables: %w", err)
		}

		l.Debugf("Route tables found with VPC ID %s: %d", p.VPCID, len(result.RouteTables))

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

		// Check if VPC has the andaime tag but is older than 1 hour
		isAndaimeVPC := false
		for _, tag := range vpc.Tags {
			if aws.ToString(tag.Key) == "andaime" {
				isAndaimeVPC = true
				break
			}
		}

		if isAndaimeVPC {
			// Delete the VPC
			_, err := p.EC2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
				VpcId: vpc.VpcId,
			})
			if err != nil {
				l.Warnf("Failed to delete VPC %s: %v", aws.ToString(vpc.VpcId), err)
				continue
			}
			l.Infof("Successfully deleted abandoned VPC %s", aws.ToString(vpc.VpcId))
		}
	}

	return nil
}

func (p *AWSProvider) waitForVPCAvailable(ctx context.Context) error {
	l := logger.Get()
	l.Debugf("Waiting for VPC to become available with ID %s", p.VPCID)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute
	b.InitialInterval = 2 * time.Second
	b.MaxInterval = 30 * time.Second

	operation := func() error {
		if err := ctx.Err(); err != nil {
			return backoff.Permanent(err)
		}

		input := &ec2.DescribeVpcsInput{
			VpcIds: []string{p.VPCID},
		}

		result, err := p.EC2Client.DescribeVpcs(ctx, input)
		if err != nil {
			l.Debugf("Failed to describe VPC with ID %s: %v", p.VPCID, err)
			return fmt.Errorf("failed to describe VPC: %w", err)
		}

		if len(result.Vpcs) > 0 {
			l.Debugf("VPC status check with ID %s: %s", p.VPCID, string(result.Vpcs[0].State))

			if result.Vpcs[0].State == ec2_types.VpcStateAvailable {
				l.Debugf("VPC is now available with ID %s", p.VPCID)
				return nil
			}
		}

		l.Debugf("VPC not yet available with ID %s", p.VPCID)
		return fmt.Errorf("VPC not yet available")
	}

	err := backoff.Retry(operation, backoff.WithContext(b, ctx))
	if err != nil {
		return fmt.Errorf("timeout waiting for VPC to become available: %w", err)
	}

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

	// Get all deployments from config
	deployments := viper.GetStringMap("deployments")
	for uniqueID, deploymentDetails := range deployments {
		details, ok := deploymentDetails.(map[string]interface{})
		if !ok || details["provider"] != "aws" {
			continue
		}

		awsDetails, ok := details["aws"].(map[string]interface{})
		if !ok {
			delete(deployments, uniqueID)
			continue
		}

		vpcID, ok := awsDetails["vpc_id"].(string)
		if !ok || vpcID == "" {
			// No VPC ID, remove this deployment
			delete(deployments, uniqueID)
			continue
		}

		l.Infof("Checking VPC %s for deployment %s", vpcID, uniqueID)

		// Check if VPC exists
		input := &ec2.DescribeVpcsInput{
			VpcIds: []string{vpcID},
		}
		result, err := p.EC2Client.DescribeVpcs(ctx, input)
		if err != nil {
			// VPC not found or error, remove deployment
			delete(deployments, uniqueID)
			continue
		}

		if len(result.Vpcs) == 0 {
			// No VPC found, remove deployment
			delete(deployments, uniqueID)
			continue
		}

		// Check if VPC is empty (no instances, subnets, etc.)
		subnets, err := p.EC2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			Filters: []ec2_types.Filter{
				{
					Name:   aws.String("vpc-id"),
					Values: []string{vpcID},
				},
			},
		})
		if err != nil {
			l.Warnf("Failed to describe subnets in VPC %s: %v", vpcID, err)
			continue
		}

		// Terminate any instances in the VPC
		instances, err := p.EC2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []ec2_types.Filter{
				{
					Name:   aws.String("vpc-id"),
					Values: []string{vpcID},
				},
			},
		})
		if err != nil {
			l.Warnf("Failed to describe instances in VPC %s: %v", vpcID, err)
			continue
		}

		// If no subnets or instances, delete the VPC
		if len(subnets.Subnets) == 0 && len(instances.Reservations) == 0 {
			l.Infof("VPC %s is empty, deleting", vpcID)
			_, err = p.EC2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
				VpcId: aws.String(vpcID),
			})
			if err != nil {
				l.Warnf("Failed to delete empty VPC %s: %v", vpcID, err)
			}
			delete(deployments, uniqueID)
			continue
		}

		// Terminate instances
		for _, reservation := range instances.Reservations {
			for _, instance := range reservation.Instances {
				if instance.State.Name != ec2_types.InstanceStateNameTerminated {
					l.Infof("Terminating instance %s", *instance.InstanceId)
					_, err := p.EC2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
						InstanceIds: []string{*instance.InstanceId},
					})
					if err != nil {
						l.Warnf(
							"Failed to terminate instance %s: %v",
							*instance.InstanceId,
							err,
						)
					}
				}
			}
		}

		// Delete the VPC
		_, err = p.EC2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
		if err != nil {
			l.Warnf("Failed to delete VPC %s: %v", vpcID, err)
			continue
		}

		// Remove this deployment from config
		delete(deployments, uniqueID)
	}

	// Update config
	viper.Set("deployments", deployments)
	if err := viper.WriteConfig(); err != nil {
		l.Warnf("Failed to update config: %v", err)
	}

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

func (p *AWSProvider) GetRegion() string {
	return p.Region
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
