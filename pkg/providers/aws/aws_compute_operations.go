package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// LiveEC2Client implements the EC2Clienter interface
type LiveEC2Client struct {
	client *ec2.Client
}

// NewEC2Client creates a new EC2 client
func NewEC2Client(ctx context.Context) (aws_interface.EC2Clienter, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &LiveEC2Client{client: ec2.NewFromConfig(cfg)}, nil
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

const (
	SSHRetryInterval = 10 * time.Second
	MaxRetries       = 30
)

func (p *AWSProvider) DeployVMsInParallel(
	ctx context.Context,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}
	deployment := m.Deployment

	eg := errgroup.Group{}

	// Group machines by region
	machinesByRegion := make(map[string][]models.Machiner)
	for _, machine := range deployment.Machines {
		region := machine.GetLocation()
		if region == "" {
			continue
		}
		// Convert zone to region if necessary
		if len(region) > 0 && region[len(region)-1] >= 'a' && region[len(region)-1] <= 'z' {
			region = region[:len(region)-1]
		}
		machinesByRegion[region] = append(machinesByRegion[region], machine)
	}

	// Deploy machines in each region
	for region, machines := range machinesByRegion {
		vpc, exists := deployment.AWS.RegionalResources.VPCs[region]
		if !exists {
			return fmt.Errorf("VPC not found for region %s", region)
		}

		// Create EC2 client for the region
		ec2Client, err := p.getOrCreateEC2Client(ctx, region)
		if err != nil {
			return fmt.Errorf("failed to create EC2 client for region %s: %w", region, err)
		}

		// Deploy machines in parallel within each region
		for _, machine := range machines {
			eg.Go(func() error {
				if err := p.deployVM(ctx, ec2Client, machine, vpc); err != nil {
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
	ec2Client aws_interface.EC2Clienter,
	machine models.Machiner,
	vpc *models.AWSVPC,
) error {
	// Get AMI ID for the region
	amiID, err := p.getLatestAMI(ctx, ec2Client)
	if err != nil {
		return fmt.Errorf("failed to get latest AMI: %w", err)
	}

	// Create instance
	runResult, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		InstanceType: ec2_types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		NetworkInterfaces: []ec2_types.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int32(0),
				SubnetId:                 aws.String(vpc.SubnetID),
				Groups:                   []string{vpc.SecurityGroupID},
				AssociatePublicIpAddress: aws.Bool(true),
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
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to run instance: %w", err)
	}

	// Wait for instance to be running
	waiter := ec2.NewInstanceRunningWaiter(ec2Client)
	if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{*runResult.Instances[0].InstanceId},
	}, 5*time.Minute); err != nil {
		return fmt.Errorf("failed waiting for instance to be running: %w", err)
	}

	// Get instance details
	describeResult, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{*runResult.Instances[0].InstanceId},
	})
	if err != nil {
		return fmt.Errorf("failed to describe instance: %w", err)
	}

	instance := describeResult.Reservations[0].Instances[0]
	machine.SetPublicIP(*instance.PublicIpAddress)
	machine.SetPrivateIP(*instance.PrivateIpAddress)

	return nil
}

func (p *AWSProvider) getLatestAMI(
	ctx context.Context,
	ec2Client aws_interface.EC2Clienter,
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

func (p *AWSProvider) createRegionalInfrastructure(
	ctx context.Context,
	region string,
	client aws_interface.EC2Clienter,
) (*models.AWSVPC, error) {
	vpc := &models.AWSVPC{}

	// Create VPC
	createVpcOutput, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
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
		return nil, fmt.Errorf("failed to create VPC in region %s: %w", region, err)
	}
	vpc.VPCID = *createVpcOutput.Vpc.VpcId

	// Create security group
	sgOutput, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("andaime-sg-%s", region)),
		VpcId:       aws.String(vpc.VPCID),
		Description: aws.String("Security group for Andaime deployments"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}
	vpc.SecurityGroupID = *sgOutput.GroupId

	// Configure security group rules
	sgRules := []ec2_types.IpPermission{}
	allowedPorts := []int32{22, 1234, 1235, 4222}
	for _, port := range allowedPorts {
		sgRules = append(sgRules, ec2_types.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(port),
			ToPort:     aws.Int32(port),
			IpRanges: []ec2_types.IpRange{
				{CidrIp: aws.String("0.0.0.0/0")},
			},
		})
	}

	_, err = client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       aws.String(vpc.SecurityGroupID),
		IpPermissions: sgRules,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to authorize security group ingress: %w", err)
	}

	// Create subnets
	zones, err := client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get availability zones: %w", err)
	}

	for i, az := range zones.AvailabilityZones {
		// Create public subnet
		publicSubnet, err := client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            aws.String(vpc.VPCID),
			CidrBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", i*2)), //nolint:mnd
			AvailabilityZone: az.ZoneName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create public subnet: %w", err)
		}
		vpc.PublicSubnetIDs = append(vpc.PublicSubnetIDs, *publicSubnet.Subnet.SubnetId)
	}

	// Create and attach internet gateway
	igw, err := client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to create internet gateway: %w", err)
	}
	vpc.InternetGatewayID = *igw.InternetGateway.InternetGatewayId

	_, err = client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(vpc.InternetGatewayID),
		VpcId:             aws.String(vpc.VPCID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach internet gateway: %w", err)
	}

	// Create and configure route table
	rt, err := client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpc.VPCID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create route table: %w", err)
	}
	vpc.PublicRouteTableID = *rt.RouteTable.RouteTableId

	// Add route to internet gateway
	_, err = client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(vpc.PublicRouteTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(vpc.InternetGatewayID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create route to internet gateway: %w", err)
	}

	// Associate public subnets with route table
	for _, subnetID := range vpc.PublicSubnetIDs {
		_, err = client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(vpc.PublicRouteTableID),
			SubnetId:     aws.String(subnetID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to associate subnet with route table: %w", err)
		}
	}

	return vpc, nil
}

func (p *AWSProvider) saveRegionalVPCToConfig(region string,
	vpc *models.AWSVPC) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	deploymentPath := fmt.Sprintf("deployments.%s.aws.regions.%s", m.Deployment.UniqueID, region)
	viper.Set(fmt.Sprintf("%s.vpc_id", deploymentPath), vpc.VPCID)
	viper.Set(fmt.Sprintf("%s.security_group_id", deploymentPath), vpc.SecurityGroupID)
	viper.Set(fmt.Sprintf("%s.public_subnet_ids", deploymentPath), vpc.PublicSubnetIDs)
	viper.Set(fmt.Sprintf("%s.private_subnet_ids", deploymentPath), vpc.PrivateSubnetIDs)
	viper.Set(fmt.Sprintf("%s.internet_gateway_id", deploymentPath), vpc.InternetGatewayID)
	viper.Set(fmt.Sprintf("%s.public_route_table_id", deploymentPath), vpc.PublicRouteTableID)

	return viper.WriteConfig()
}

func (p *AWSProvider) waitForSSHConnectivity(ctx context.Context, machine models.Machiner) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	sshConfig, err := sshutils.NewSSHConfigFunc(
		machine.GetPublicIP(),
		machine.GetSSHPort(),
		machine.GetSSHUser(),
		machine.GetSSHPrivateKeyPath(),
	)

	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	for i := 0; i < MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := sshConfig.WaitForSSH(ctx, MaxRetries, SSHRetryInterval)
			if err == nil {
				l.Infof("SSH connectivity established for VM %s", machine.GetName())
				machine.SetMachineResourceState("SSH", models.ResourceStateSucceeded)
				return nil
			}

			l.Debugf("Waiting for SSH on VM %s (attempt %d/%d): %v",
				machine.GetName(), i+1, MaxRetries, err)
			machine.SetMachineResourceState("SSH", models.ResourceStatePending)

			// Update the model with current status
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.AWSResourceTypeInstance,
				models.ResourceStatePending,
				fmt.Sprintf("Waiting for SSH connectivity (attempt %d/%d)", i+1, MaxRetries),
			))

			time.Sleep(SSHRetryInterval)
		}
	}

	return fmt.Errorf(
		"SSH connectivity timeout after %d attempts for VM %s",
		MaxRetries,
		machine.GetName(),
	)
}
