package awsprovider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func (p *AWSProvider) CreateVPC(ctx context.Context) error {
	l := logger.Get()
	l.Info("Creating VPC...")

	if p.EC2Client == nil {
		return fmt.Errorf("EC2 client is not initialized")
	}

	// Create VPC
	createVpcInput := &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("AndaimeVPC"),
					},
				},
			},
		},
	}

	vpcOutput, err := p.EC2Client.CreateVPC(ctx, createVpcInput)
	if err != nil {
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	p.VPCID = *vpcOutput.Vpc.VpcId

	// Create subnets
	azs, err := p.EC2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return fmt.Errorf("failed to get availability zones: %w", err)
	}

	// Create public subnet
	createPublicSubnetInput := &ec2.CreateSubnetInput{
		VpcId:            aws.String(p.VPCID),
		CidrBlock:        aws.String("10.0.1.0/24"),
		AvailabilityZone: azs.AvailabilityZones[0].ZoneName,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("AndaimePublicSubnet"),
					},
				},
			},
		},
	}

	publicSubnet, err := p.EC2Client.CreateSubnet(ctx, createPublicSubnetInput)
	if err != nil {
		return fmt.Errorf("failed to create public subnet: %w", err)
	}

	// Create private subnet
	createPrivateSubnetInput := &ec2.CreateSubnetInput{
		VpcId:            aws.String(p.VPCID),
		CidrBlock:        aws.String("10.0.2.0/24"),
		AvailabilityZone: azs.AvailabilityZones[0].ZoneName,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("AndaimePrivateSubnet"),
					},
				},
			},
		},
	}

	privateSubnet, err := p.EC2Client.CreateSubnet(ctx, createPrivateSubnetInput)
	if err != nil {
		return fmt.Errorf("failed to create private subnet: %w", err)
	}

	// Create and attach internet gateway
	createIgwInput := &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInternetGateway,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("AndaimeIGW"),
					},
				},
			},
		},
	}

	igw, err := p.EC2Client.CreateInternetGateway(ctx, createIgwInput)
	if err != nil {
		return fmt.Errorf("failed to create internet gateway: %w", err)
	}

	_, err = p.EC2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igw.InternetGateway.InternetGatewayId,
		VpcId:            aws.String(p.VPCID),
	})
	if err != nil {
		return fmt.Errorf("failed to attach internet gateway: %w", err)
	}

	// Create route table for public subnet
	createRouteTableInput := &ec2.CreateRouteTableInput{
		VpcId: aws.String(p.VPCID),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeRouteTable,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("AndaimePublicRT"),
					},
				},
			},
		},
	}

	routeTable, err := p.EC2Client.CreateRouteTable(ctx, createRouteTableInput)
	if err != nil {
		return fmt.Errorf("failed to create route table: %w", err)
	}

	// Create route to internet gateway
	_, err = p.EC2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:           igw.InternetGateway.InternetGatewayId,
	})
	if err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}

	// Associate route table with public subnet
	_, err = p.EC2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
		RouteTableId: routeTable.RouteTable.RouteTableId,
		SubnetId:     publicSubnet.Subnet.SubnetId,
	})
	if err != nil {
		return fmt.Errorf("failed to associate route table: %w", err)
	}

	l.Info("VPC and networking components created successfully")
	return nil
}
