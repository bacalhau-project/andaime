package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"

	"golang.org/x/sync/errgroup"
)

// UNKNOWN represents an unknown value in AWS responses
const UNKNOWN = "unknown"

// Network configuration constants
const (
	VPCCidrBlock          = "10.0.0.0/16"
	PublicSubnet1CIDR     = "10.0.0.0/24"
	PublicSubnet2CIDR     = "10.0.1.0/24"
	PrivateSubnet1CIDR    = "10.0.2.0/24"
	PrivateSubnet2CIDR    = "10.0.3.0/24"
	MinRequiredAZs        = 2
	SSHRetryInterval      = 10 * time.Second
	MaxRetries            = 30
	TimeToWaitForInstance = 5 * time.Minute
)

// LiveEC2Client implements the EC2Clienter interface
type LiveEC2Client struct {
	client     *ec2.Client
	imdsClient *imds.Client
}

// NewEC2Client creates a new EC2 client
func NewEC2Client(ctx context.Context) (aws_interfaces.EC2Clienter, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &LiveEC2Client{
		client:     ec2.NewFromConfig(cfg),
		imdsClient: imds.NewFromConfig(cfg),
	}, nil
}

func (c *LiveEC2Client) RunInstances(
	ctx context.Context,
	params *ec2.RunInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.RunInstancesOutput, error) {
	return c.client.RunInstances(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return c.client.DescribeInstances(ctx, params, optFns...)
}

func (c *LiveEC2Client) TerminateInstances(
	ctx context.Context,
	params *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	return c.client.TerminateInstances(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeImages(
	ctx context.Context,
	params *ec2.DescribeImagesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeImagesOutput, error) {
	return c.client.DescribeImages(ctx, params, optFns...)
}

func (c *LiveEC2Client) CreateVpc(
	ctx context.Context,
	params *ec2.CreateVpcInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateVpcOutput, error) {
	return c.client.CreateVpc(ctx, params, optFns...)
}

func (c *LiveEC2Client) CreateSubnet(
	ctx context.Context,
	params *ec2.CreateSubnetInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateSubnetOutput, error) {
	return c.client.CreateSubnet(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeAvailabilityZones(
	ctx context.Context,
	params *ec2.DescribeAvailabilityZonesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return c.client.DescribeAvailabilityZones(ctx, params, optFns...)
}

func (c *LiveEC2Client) DeleteSecurityGroup(
	ctx context.Context,
	params *ec2.DeleteSecurityGroupInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteSecurityGroupOutput, error) {
	return c.client.DeleteSecurityGroup(ctx, params, optFns...)
}

func (c *LiveEC2Client) CreateSecurityGroup(
	ctx context.Context,
	params *ec2.CreateSecurityGroupInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateSecurityGroupOutput, error) {
	return c.client.CreateSecurityGroup(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeSecurityGroups(
	ctx context.Context,
	params *ec2.DescribeSecurityGroupsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSecurityGroupsOutput, error) {
	return c.client.DescribeSecurityGroups(ctx, params, optFns...)
}

func (c *LiveEC2Client) AuthorizeSecurityGroupIngress(
	ctx context.Context,
	params *ec2.AuthorizeSecurityGroupIngressInput,
	optFns ...func(*ec2.Options),
) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return c.client.AuthorizeSecurityGroupIngress(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeSubnets(
	ctx context.Context,
	params *ec2.DescribeSubnetsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSubnetsOutput, error) {
	return c.client.DescribeSubnets(ctx, params, optFns...)
}

func (c *LiveEC2Client) DeleteVpc(
	ctx context.Context,
	params *ec2.DeleteVpcInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteVpcOutput, error) {
	return c.client.DeleteVpc(ctx, params, optFns...)
}

func (c *LiveEC2Client) CreateInternetGateway(
	ctx context.Context,
	params *ec2.CreateInternetGatewayInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateInternetGatewayOutput, error) {
	return c.client.CreateInternetGateway(ctx, params, optFns...)
}

func (c *LiveEC2Client) AttachInternetGateway(
	ctx context.Context,
	params *ec2.AttachInternetGatewayInput,
	optFns ...func(*ec2.Options),
) (*ec2.AttachInternetGatewayOutput, error) {
	return c.client.AttachInternetGateway(ctx, params, optFns...)
}

func (c *LiveEC2Client) CreateRouteTable(
	ctx context.Context,
	params *ec2.CreateRouteTableInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateRouteTableOutput, error) {
	return c.client.CreateRouteTable(ctx, params, optFns...)
}

func (c *LiveEC2Client) CreateRoute(
	ctx context.Context,
	params *ec2.CreateRouteInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateRouteOutput, error) {
	return c.client.CreateRoute(ctx, params, optFns...)
}

func (c *LiveEC2Client) AssociateRouteTable(
	ctx context.Context,
	params *ec2.AssociateRouteTableInput,
	optFns ...func(*ec2.Options),
) (*ec2.AssociateRouteTableOutput, error) {
	return c.client.AssociateRouteTable(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeRouteTables(
	ctx context.Context,
	params *ec2.DescribeRouteTablesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeRouteTablesOutput, error) {
	return c.client.DescribeRouteTables(ctx, params, optFns...)
}

func (c *LiveEC2Client) DeleteSubnet(
	ctx context.Context,
	params *ec2.DeleteSubnetInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteSubnetOutput, error) {
	return c.client.DeleteSubnet(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeVpcs(
	ctx context.Context,
	params *ec2.DescribeVpcsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	return c.client.DescribeVpcs(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeRegions(
	ctx context.Context,
	params *ec2.DescribeRegionsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeRegionsOutput, error) {
	return c.client.DescribeRegions(ctx, params, optFns...)
}

func (c *LiveEC2Client) ModifyVpcAttribute(
	ctx context.Context,
	params *ec2.ModifyVpcAttributeInput,
	optFns ...func(*ec2.Options),
) (*ec2.ModifyVpcAttributeOutput, error) {
	return c.client.ModifyVpcAttribute(ctx, params, optFns...)
}

func (c *LiveEC2Client) DisassociateRouteTable(
	ctx context.Context,
	params *ec2.DisassociateRouteTableInput,
	optFns ...func(*ec2.Options),
) (*ec2.DisassociateRouteTableOutput, error) {
	return c.client.DisassociateRouteTable(ctx, params, optFns...)
}

func (c *LiveEC2Client) DeleteRouteTable(
	ctx context.Context,
	params *ec2.DeleteRouteTableInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteRouteTableOutput, error) {
	return c.client.DeleteRouteTable(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeInternetGateways(
	ctx context.Context,
	params *ec2.DescribeInternetGatewaysInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInternetGatewaysOutput, error) {
	return c.client.DescribeInternetGateways(ctx, params, optFns...)
}

func (c *LiveEC2Client) DetachInternetGateway(
	ctx context.Context,
	params *ec2.DetachInternetGatewayInput,
	optFns ...func(*ec2.Options),
) (*ec2.DetachInternetGatewayOutput, error) {
	return c.client.DetachInternetGateway(ctx, params, optFns...)
}

func (c *LiveEC2Client) DeleteInternetGateway(
	ctx context.Context,
	params *ec2.DeleteInternetGatewayInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteInternetGatewayOutput, error) {
	return c.client.DeleteInternetGateway(ctx, params, optFns...)
}

func (p *AWSProvider) DeployVMsInParallel(
	ctx context.Context,
) error {
	l := logger.Get()
	l.Info("Starting parallel VM deployment")

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		l.Error("Global model or deployment is nil")
		return fmt.Errorf("global model or deployment is nil")
	}

	// Log deployment details
	l.Info(fmt.Sprintf("Deploying %d machines", len(m.Deployment.GetMachines())))

	// Group machines by region
	machinesByRegion := make(map[string][]models.Machiner)
	for _, machine := range m.Deployment.GetMachines() {
		region := machine.GetRegion()
		l.Debug(fmt.Sprintf("Processing machine %s in region %s", machine.GetName(), region))
		if region == "" {
			l.Warn(fmt.Sprintf("Machine %s has no location specified", machine.GetName()))
			continue
		}
		// Convert zone to region if necessary
		if len(region) > 0 && region[len(region)-1] >= 'a' && region[len(region)-1] <= 'z' {
			region = region[:len(region)-1]
		}
		machinesByRegion[region] = append(machinesByRegion[region], machine)
	}

	// Validate zones for all regions
	l.Info("Validating availability zones for all regions")
	for region := range machinesByRegion {
		l.Debug(fmt.Sprintf("Validating region: %s", region))
		if err := p.validateRegionZones(ctx, region); err != nil {
			l.Error(fmt.Sprintf("Zone validation failed for region %s: %v", region, err))
			return fmt.Errorf("zone validation failed: %w", err)
		}
	}

	eg := errgroup.Group{}

	// Deploy machines in each region
	for region, machines := range machinesByRegion {
		vpc := m.Deployment.AWS.RegionalResources.GetVPC(region)
		if vpc == nil {
			return fmt.Errorf("VPC not found for region during deployment: %s", region)
		}

		// Create EC2 client for the region
		regionalClient, err := p.GetOrCreateEC2Client(ctx, region)
		if err != nil {
			return fmt.Errorf("failed to create EC2 client for region %s: %w", region, err)
		}

		// Deploy machines in parallel within each region
		for _, machine := range machines {
			eg.Go(func() error {
				if err := p.deployVM(ctx, regionalClient, machine, vpc); err != nil {
					return fmt.Errorf(
						"failed to deploy VM %s in region %s: %w",
						machine.GetName(),
						region,
						err,
					)
				}
				return nil
			})
		}
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (p *AWSProvider) deployVM(
	ctx context.Context,
	ec2Client aws_interfaces.EC2Clienter,
	machine models.Machiner,
	vpc *models.AWSVPC,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	// Validate VPC configuration
	if vpc == nil {
		return fmt.Errorf("vpc is nil")
	}
	if len(vpc.PublicSubnetIDs) == 0 {
		return fmt.Errorf("no public subnets available in VPC %s", vpc.VPCID)
	}
	if vpc.SecurityGroupID == "" {
		return fmt.Errorf("no security group ID set for VPC %s", vpc.VPCID)
	}

	// Get AMI ID for the region
	amiID, err := p.GetLatestUbuntuAMI(ctx, machine.GetRegion(), "x86_64")
	if err != nil {
		return fmt.Errorf("failed to get latest AMI: %w", err)
	}

	// Use the first public subnet
	subnetID := vpc.PublicSubnetIDs[0]
	l.Debug(fmt.Sprintf("Using subnet %s for VM %s", subnetID, machine.GetName()))

	// Prepare tag specifications
	var tagSpecs []ec2_types.TagSpecification
	for k, v := range m.Deployment.Tags {
		tagSpecs = append(tagSpecs, ec2_types.TagSpecification{
			ResourceType: ec2_types.ResourceTypeInstance,
			Tags:         []ec2_types.Tag{{Key: aws.String(k), Value: aws.String(v)}},
		})
	}

	tagSpecs = append(tagSpecs, ec2_types.TagSpecification{
		ResourceType: ec2_types.ResourceTypeInstance,
		Tags: []ec2_types.Tag{
			{Key: aws.String("Name"), Value: aws.String(machine.GetName())},
			{Key: aws.String("AndaimeMachineID"), Value: aws.String(machine.GetID())},
			{Key: aws.String("AndaimeDeployment"), Value: aws.String("AndaimeDeployment")},
		},
	})

	// Prepare user data script
	userData := fmt.Sprintf(`#!/bin/bash
SSH_USERNAME=%s
SSH_PUBLIC_KEY_MATERIAL=%q

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
		machine.GetSSHUser(),
		machine.GetSSHPublicKeyMaterial(),
	)

	// Encode the script in base64
	encodedUserData := base64.StdEncoding.EncodeToString([]byte(userData))

	// Prepare instance input with common configuration
	params := machine.GetParameters()
	var runResult *ec2.RunInstancesOutput
	runInstancesInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		InstanceType: ec2_types.InstanceType(machine.GetMachineType()),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		NetworkInterfaces: []ec2_types.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int32(0),
				SubnetId:                 aws.String(subnetID),
				Groups:                   []string{vpc.SecurityGroupID},
				AssociatePublicIpAddress: aws.Bool(true),
			},
		},
		TagSpecifications: tagSpecs,
		UserData:          aws.String(encodedUserData),
	}

	// Configure spot instance if requested
	if params.Spot {
		l.Info(fmt.Sprintf("Requesting spot instance for VM %s", machine.GetName()))
		maxRetries := 3
		var lastErr error

		for i := 0; i < maxRetries; i++ {
			runInstancesInput.InstanceMarketOptions = &ec2_types.InstanceMarketOptionsRequest{
				MarketType: ec2_types.MarketTypeSpot,
				SpotOptions: &ec2_types.SpotMarketOptions{
					InstanceInterruptionBehavior: ec2_types.InstanceInterruptionBehaviorTerminate,
					MaxPrice: aws.String(
						"",
					), // Empty string means use current spot price
				},
			}

			runResult, err = ec2Client.RunInstances(ctx, runInstancesInput)
			if err == nil {
				break
			}

			lastErr = err
			l.Warn(
				fmt.Sprintf(
					"Spot instance request failed (attempt %d/%d): %v",
					i+1,
					maxRetries,
					err,
				),
			)

			// Check if error is related to spot capacity
			if i < maxRetries-1 {
				time.Sleep(time.Second * time.Duration(2<<uint(i))) // Exponential backoff
			}
		}

		if lastErr != nil {
			// If all spot attempts failed, try falling back to on-demand
			l.Warn(
				fmt.Sprintf(
					"All spot instance attempts failed for %s, falling back to on-demand: %v",
					machine.GetName(),
					lastErr,
				),
			)
			runInstancesInput.InstanceMarketOptions = nil
			runResult, err = ec2Client.RunInstances(ctx, runInstancesInput)
			if err != nil {
				return fmt.Errorf("failed to run instance (both spot and on-demand): %w", err)
			}
		} else {
			return fmt.Errorf("failed to run instance: %w", err)
		}
		if params.Spot {
			// If spot instance creation fails, try falling back to on-demand
			l.Warn(fmt.Sprintf(
				"Failed to create spot instance for VM %s, falling back to on-demand: %v",
				machine.GetName(),
				err,
			))
			runInstancesInput.InstanceMarketOptions = nil
			runResult, err = ec2Client.RunInstances(ctx, runInstancesInput)
			if err != nil {
				return fmt.Errorf("failed to create fallback on-demand instance: %w", err)
			}
		} else {
		}
	} else {
		runResult, err = ec2Client.RunInstances(ctx, runInstancesInput)
		if err != nil {
			return fmt.Errorf("failed to run instance: %w", err)
		}
	}

	// Wait for instance to be running with enhanced status checks
	waiter := ec2.NewInstanceRunningWaiter(ec2Client)
	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{*runResult.Instances[0].InstanceId},
	}
	err = waiter.Wait(ctx, describeInput, TimeToWaitForInstance)
	if err != nil {
		return fmt.Errorf("failed waiting for instance to be running: %w", err)
	}

	// Get instance details with retries
	var describeResult *ec2.DescribeInstancesOutput
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		describeResult, err = ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{*runResult.Instances[0].InstanceId},
		}, func(o *ec2.Options) {})
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(2<<uint(i)))
		}
	}
	if err != nil {
		return fmt.Errorf("failed to describe instance after %d attempts: %w", maxRetries, err)
	}

	if len(describeResult.Reservations) == 0 {
		return fmt.Errorf("no reservations found for instance")
	}
	if len(describeResult.Reservations[0].Instances) == 0 {
		return fmt.Errorf("no instances found in reservation")
	}

	instance := describeResult.Reservations[0].Instances[0]

	// Get instance metadata using IMDS client if available
	if c, ok := ec2Client.(*LiveEC2Client); ok && c.imdsClient != nil {
		// Attempt to get additional metadata but don't fail if unavailable
		if metadata, err := c.imdsClient.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{}); err == nil {
			l.Debug(
				fmt.Sprintf(
					"Instance metadata: region=%s, instanceType=%s",
					metadata.Region,
					metadata.InstanceType,
				),
			)
		}
	}

	if instance.PublicIpAddress != nil {
		machine.SetPublicIP(*instance.PublicIpAddress)
	}
	if instance.PrivateIpAddress != nil {
		machine.SetPrivateIP(*instance.PrivateIpAddress)
	}

	return nil
}

func (p *AWSProvider) getLatestAMI(
	ctx context.Context,
	ec2Client aws_interfaces.EC2Clienter,
) (string, error) {
	result, err := ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"amzn2-ami-hvm-*-x86_64-gp2"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
		Owners: []string{"amazon"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe images: %w", err)
	}

	// Find the latest AMI
	var latestImage *ec2_types.Image
	for i := range result.Images {
		if latestImage == nil || *result.Images[i].CreationDate > *latestImage.CreationDate {
			latestImage = &result.Images[i]
		}
	}

	if latestImage == nil {
		return "", fmt.Errorf("no AMI found")
	}

	return *latestImage.ImageId, nil
}

func (p *AWSProvider) validateRegionZones(ctx context.Context, region string) error {
	l := logger.Get()
	l.Info(fmt.Sprintf("Validating availability zones for region: %s", region))

	ec2Client, err := p.GetOrCreateEC2Client(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to create EC2 client for region %s: %w", region, err)
	}

	// Describe all zones in the region, removing filters to get comprehensive data
	result, err := ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
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
		AllAvailabilityZones: aws.Bool(true),
	})
	if err != nil {
		l.Error(fmt.Sprintf("Failed to describe AZs: %v", err))
		return fmt.Errorf("failed to describe availability zones: %w", err)
	}

	// Log all found zones with detailed information
	l.Info(fmt.Sprintf("Found total of %d zones:", len(result.AvailabilityZones)))
	var availableAZs []string

	for _, az := range result.AvailabilityZones {
		// Safely handle potential nil values
		zoneName := UNKNOWN
		if az.ZoneName != nil {
			zoneName = *az.ZoneName
		}

		zoneType := UNKNOWN
		if az.ZoneType != nil {
			zoneType = *az.ZoneType
		}

		regionName := UNKNOWN
		if az.RegionName != nil {
			regionName = *az.RegionName
		}

		zoneInfo := fmt.Sprintf(
			"Zone: %s, State: %s, Type: %s, Region: %s",
			zoneName,
			string(az.State),
			zoneType,
			regionName,
		)
		l.Debug(zoneInfo)

		// Explicitly check for region and availability
		if regionName == region && az.State == ec2_types.AvailabilityZoneStateAvailable {
			availableAZs = append(availableAZs, zoneName)
		}
	}

	l.Info(
		fmt.Sprintf("Found %d available zones in %s: %v", len(availableAZs), region, availableAZs),
	)

	if len(availableAZs) < MinRequiredAZs {
		return fmt.Errorf(
			"region %s does not have at least %d availability zones (found %d: %v)",
			region,
			MinRequiredAZs,
			len(availableAZs),
			availableAZs,
		)
	}

	l.Info(fmt.Sprintf(
		"Successfully validated region %s has %d availability zones: %v",
		region,
		len(availableAZs),
		availableAZs,
	))
	return nil
}

// GetIMDSConfig returns the IMDS client for EC2 instance metadata
func (c *LiveEC2Client) GetIMDSConfig() *imds.Client {
	return c.imdsClient
}
