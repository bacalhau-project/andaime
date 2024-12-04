package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/logger"
	aws_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"
	"github.com/spf13/viper"
)

// Destroy updated to clean up resources across all regions
func (p *AWSProvider) Destroy(ctx context.Context, deploymentID string) error {
	l := logger.Get()
	l.Info("Starting destruction of AWS resources across all regions")

	deployments := viper.GetStringMap("deployments.aws")
	if deployments == nil {
		return nil
	}

	for uniqueID, deploymentDetails := range deployments {
		details, ok := deploymentDetails.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle both legacy and new configurations
		if regions, ok := details["regions"].(map[string]interface{}); ok {
			// New configuration with regions
			for region, regionDetails := range regions {
				rd, ok := regionDetails.(map[string]interface{})
				if !ok {
					continue
				}

				vpcID, ok := rd["vpc_id"].(string)
				if !ok {
					continue
				}

				if err := p.cleanupRegionalResources(ctx, region, vpcID); err != nil {
					l.Warnf("Failed to clean up resources in region %s: %v", region, err)
				}
			}
		} else {
			// Legacy configuration with single region
			region, ok := details["region"].(string)
			if !ok {
				continue
			}

			vpcID, _ := details["vpc_id"].(string)
			if err := p.cleanupRegionalResources(ctx, region, vpcID); err != nil {
				l.Warnf("Failed to clean up resources in region %s: %v", region, err)
			}
		}

		// Remove deployment from config
		delete(deployments, uniqueID)
	}

	// Update config
	viper.Set("deployments.aws", deployments)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to update config: %v", err)
	}

	return nil
}

func (p *AWSProvider) cleanupRegionalResources(ctx context.Context, region, vpcID string) error {
	if vpcID == "" {
		return nil
	}

	client, err := p.getOrCreateEC2Client(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to create client for region %s: %w", region, err)
	}

	return p.destroyRegionalResources(ctx, region, vpcID, client)
}

// destroyRegionalResources cleans up all resources in a specific region
func (p *AWSProvider) destroyRegionalResources(
	ctx context.Context,
	region string,
	vpcID string,
	client aws_interface.EC2Clienter,
) error {
	l := logger.Get()
	l.Infof("Destroying resources in region %s", region)

	// First, terminate all instances in the VPC
	instances, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe instances: %w", err)
	}

	// Terminate instances
	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			if instance.State.Name != ec2_types.InstanceStateNameTerminated {
				_, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
					InstanceIds: []string{*instance.InstanceId},
				})
				if err != nil {
					l.Warnf("Failed to terminate instance %s: %v", *instance.InstanceId, err)
				}
			}
		}
	}

	// Get VPC details
	vpcDetails, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil {
		return fmt.Errorf("failed to describe VPC: %w", err)
	}

	if len(vpcDetails.Vpcs) == 0 {
		return fmt.Errorf("VPC %s not found", vpcID)
	}

	// Clean up in reverse order of creation
	// 1. Delete subnets
	subnets, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		l.Warnf("Failed to describe subnets: %v", err)
	} else {
		for _, subnet := range subnets.Subnets {
			_, err := client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
				SubnetId: subnet.SubnetId,
			})
			if err != nil {
				l.Warnf("Failed to delete subnet %s: %v", *subnet.SubnetId, err)
			}
		}
	}

	// 2. Delete security groups
	sgs, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		l.Warnf("Failed to describe security groups: %v", err)
	} else {
		for _, sg := range sgs.SecurityGroups {
			if aws.ToString(sg.GroupName) != DefaultName {
				_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
					GroupId: sg.GroupId,
				})
				if err != nil {
					l.Warnf("Failed to delete security group %s: %v", *sg.GroupId, err)
				}
			}
		}
	}

	// 3. Delete VPC
	_, err = client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}

	l.Infof("Successfully destroyed resources in region %s", region)
	return nil
}
