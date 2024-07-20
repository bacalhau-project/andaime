package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type AWSProvider struct {
	Config    aws.Config
	EC2Client EC2ClientInterface
}

func NewAWSProvider() (AWSProviderInterface, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	return &AWSProvider{
		Config:    cfg,
		EC2Client: ec2.NewFromConfig(cfg),
	}, nil
}

func (p *AWSProvider) GetConfig() aws.Config {
	return p.Config
}

func (p *AWSProvider) SetConfig(config aws.Config) {
	p.Config = config
}

func (p *AWSProvider) GetEC2Client() (EC2ClientInterface, error) {
	return p.EC2Client, nil
}

func (p *AWSProvider) SetEC2Client(client EC2ClientInterface) {
	p.EC2Client = client
}

func (p *AWSProvider) CreateCluster(clusterName string, numMachines int) error {
	vpc, err := p.CreateVPC(clusterName)
	if err != nil {
		return fmt.Errorf("failed to create VPC: %v", err)
	}

	subnet, err := p.CreateSubnet(vpc.VpcId, clusterName)
	if err != nil {
		return fmt.Errorf("failed to create subnet: %v", err)
	}

	sg, err := p.CreateSecurityGroup(vpc.VpcId, clusterName)
	if err != nil {
		return fmt.Errorf("failed to create security group: %v", err)
	}

	for i := 0; i < numMachines; i++ {
		instanceName := fmt.Sprintf("%s-instance-%d", clusterName, i)
		_, err := p.CreateEC2Instance(subnet.SubnetId, sg.GroupId, instanceName)
		if err != nil {
			return fmt.Errorf("failed to create EC2 instance %s: %v", instanceName, err)
		}
	}

	return nil
}

func (p *AWSProvider) CreateVPC(name string) (*types.Vpc, error) {
	input := &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("andaime"), Value: aws.String("true")},
				},
			},
		},
	}

	result, err := p.EC2Client.CreateVpc(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	return result.Vpc, nil
}

func (p *AWSProvider) CreateSubnet(vpcID *string, name string) (*types.Subnet, error) {
	input := &ec2.CreateSubnetInput{
		VpcId:     vpcID,
		CidrBlock: aws.String("10.0.1.0/24"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("andaime"), Value: aws.String("true")},
				},
			},
		},
	}

	result, err := p.EC2Client.CreateSubnet(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	return result.Subnet, nil
}

func (p *AWSProvider) CreateSecurityGroup(vpcID *string, name string) (*types.SecurityGroup, error) {
	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: aws.String("Security group for Andaime cluster"),
		VpcId:       vpcID,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("andaime"), Value: aws.String("true")},
				},
			},
		},
	}

	result, err := p.EC2Client.CreateSecurityGroup(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	// Add inbound rules for SSH and internal communication
	_, err = p.EC2Client.AuthorizeSecurityGroupIngress(context.TODO(), &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: result.GroupId,
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
			},
			{
				IpProtocol: aws.String("-1"),
				FromPort:   aws.Int32(-1),
				ToPort:     aws.Int32(-1),
				UserIdGroupPairs: []types.UserIdGroupPair{
					{GroupId: result.GroupId},
				},
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to add security group rules: %v", err)
	}

	return &types.SecurityGroup{GroupId: result.GroupId}, nil
}

func (p *AWSProvider) CreateEC2Instance(subnetID, securityGroupID *string, name string) (*types.Instance, error) {
	input := &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-0c55b159cbfafe1f0"), // Amazon Linux 2 AMI (HVM), SSD Volume Type
		InstanceType: types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		SubnetId:     subnetID,
		SecurityGroupIds: []string{
			*securityGroupID,
		},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("andaime"), Value: aws.String("true")},
				},
			},
		},
	}

	result, err := p.EC2Client.RunInstances(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	return &result.Instances[0], nil
}

func (p *AWSProvider) ListResources() error {
	// List EC2 instances
	instancesResult, err := p.EC2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list EC2 instances: %v", err)
	}

	for _, reservation := range instancesResult.Reservations {
		for _, instance := range reservation.Instances {
			fmt.Printf("EC2 Instance: %s, State: %s\n", *instance.InstanceId, instance.State.Name)
		}
	}

	// List VPCs
	vpcsResult, err := p.EC2Client.DescribeVpcs(context.TODO(), &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list VPCs: %v", err)
	}

	for _, vpc := range vpcsResult.Vpcs {
		fmt.Printf("VPC: %s\n", *vpc.VpcId)
	}

	// List Subnets
	subnetsResult, err := p.EC2Client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list Subnets: %v", err)
	}

	for _, subnet := range subnetsResult.Subnets {
		fmt.Printf("Subnet: %s, VPC: %s\n", *subnet.SubnetId, *subnet.VpcId)
	}

	// List Security Groups
	sgResult, err := p.EC2Client.DescribeSecurityGroups(context.TODO(), &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list Security Groups: %v", err)
	}

	for _, sg := range sgResult.SecurityGroups {
		fmt.Printf("Security Group: %s, VPC: %s\n", *sg.GroupId, *sg.VpcId)
	}

	return nil
}

func (p *AWSProvider) DestroyResources(ctx context.Context, client *ec2.Client) error {
	client.DeleteVpc(context.TODO(), &ec2.DeleteVpcInput{})

	// Terminate EC2 instances
	instancesResult, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list EC2 instances: %v", err)
	}

	for _, reservation := range instancesResult.Reservations {
		for _, instance := range reservation.Instances {
			_, err := p.EC2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
				InstanceIds: []string{*instance.InstanceId},
			})
			if err != nil {
				log.Printf("Failed to terminate instance %s: %v", *instance.InstanceId, err)
			} else {
				fmt.Printf("Terminated EC2 Instance: %s\n", *instance.InstanceId)
			}
		}
	}

	// Delete Security Groups
	sgResult, err := p.EC2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list Security Groups: %v", err)
	}

	for _, sg := range sgResult.SecurityGroups {
		_, err := p.EC2Client.DeleteSecurityGroup(context.TODO(), &ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		})
		if err != nil {
			log.Printf("Failed to delete Security Group %s: %v", *sg.GroupId, err)
		} else {
			fmt.Printf("Deleted Security Group: %s\n", *sg.GroupId)
		}
	}

	// Delete Subnets
	subnetsResult, err := p.EC2Client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list Subnets: %v", err)
	}

	for _, subnet := range subnetsResult.Subnets {
		_, err := p.EC2Client.DeleteSubnet(context.TODO(), &ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		})
		if err != nil {
			log.Printf("Failed to delete Subnet %s: %v", *subnet.SubnetId, err)
		} else {
			fmt.Printf("Deleted Subnet: %s\n", *subnet.SubnetId)
		}
	}

	// Delete VPCs
	vpcsResult, err := p.EC2Client.DescribeVpcs(context.TODO(), &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:andaime"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list VPCs: %v", err)
	}

	for _, vpc := range vpcsResult.Vpcs {
		_, err := p.EC2Client.DeleteVpc(context.TODO(), &ec2.DeleteVpcInput{
			VpcId: vpc.VpcId,
		})
		if err != nil {
			log.Printf("Failed to delete VPC %s: %v", *vpc.VpcId, err)
		} else {
			fmt.Printf("Deleted VPC: %s\n", *vpc.VpcId)
		}
	}

	return nil
}
