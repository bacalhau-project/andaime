package awsprovider

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// LiveEC2Client implements the EC2Clienter interface
type LiveEC2Client struct {
	client *ec2.Client
}

func (c *LiveEC2Client) WaitUntilInstanceRunning(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) error {
	waiter := ec2.NewInstanceRunningWaiter(c.client)
	return waiter.Wait(ctx, params, 5*time.Minute, optFns...)
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

func (c *LiveEC2Client) DescribeAvailabilityZones(
	ctx context.Context,
	params *ec2.DescribeAvailabilityZonesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return c.client.DescribeAvailabilityZones(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeRouteTables(
	ctx context.Context,
	params *ec2.DescribeRouteTablesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeRouteTablesOutput, error) {
	return c.client.DescribeRouteTables(ctx, params, optFns...)
}

func (c *LiveEC2Client) DescribeVpcs(
	ctx context.Context,
	params *ec2.DescribeVpcsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	return c.client.DescribeVpcs(ctx, params, optFns...)
}

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	SSHRetryInterval = 10 * time.Second
	MaxRetries       = 30
)

// DeployVMsInParallel deploys multiple VMs in parallel and waits for SSH connectivity
func (p *AWSProvider) DeployVMsInParallel(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	var g errgroup.Group
	var mu sync.Mutex

	// Deploy all VMs in parallel
	for _, machine := range m.Deployment.GetMachines() {
		machine := machine // Create local copy for goroutine
		g.Go(func() error {
			// Update initial machine state
			machine.SetMachineResourceState("Instance", models.ResourceStatePending)
			machine.SetMachineResourceState("Network", models.ResourceStatePending)
			machine.SetMachineResourceState("SSH", models.ResourceStatePending)

			// Create and configure the VM
			runResult, err := p.EC2Client.RunInstances(ctx, &ec2.RunInstancesInput{
				ImageId:      aws.String(machine.GetImageID()),
				InstanceType: types.InstanceType(machine.GetType().ResourceString),
				MinCount:     aws.Int32(1),
				MaxCount:     aws.Int32(1),
				KeyName:      aws.String(m.Deployment.GetSSHKeyName()),
				NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
					{
						DeviceIndex:              aws.Int32(0),
						AssociatePublicIpAddress: aws.Bool(true),
						DeleteOnTermination:      aws.Bool(true),
						SubnetId:                 aws.String(p.SubnetID),
						Groups:                   []string{p.SecurityGroupID},
					},
				},
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeInstance,
						Tags: []types.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String(machine.GetName()),
							},
							{
								Key:   aws.String("Project"),
								Value: aws.String("andaime"),
							},
						},
					},
				},
			})
			if err != nil {
				mu.Lock()
				machine.SetFailed(true)
				machine.SetMachineResourceState("Instance", models.ResourceStateFailed)
				mu.Unlock()
				return fmt.Errorf("failed to create VM %s: %w", machine.GetName(), err)
			}

			// Wait for instance to be running
			waiterInput := &ec2.DescribeInstancesInput{
				InstanceIds: []string{*runResult.Instances[0].InstanceId},
			}
			if err := p.EC2Client.WaitUntilInstanceRunning(ctx, waiterInput); err != nil {
				mu.Lock()
				machine.SetFailed(true)
				machine.SetMachineResourceState("Instance", models.ResourceStateFailed)
				mu.Unlock()
				return fmt.Errorf("failed waiting for instance to be running: %w", err)
			}

			// Get instance details
			describeResult, err := p.EC2Client.DescribeInstances(ctx, waiterInput)
			if err != nil {
				mu.Lock()
				machine.SetFailed(true)
				machine.SetMachineResourceState("Instance", models.ResourceStateFailed)
				mu.Unlock()
				return fmt.Errorf("failed to describe instance: %w", err)
			}

			instance := describeResult.Reservations[0].Instances[0]
			machine.SetMachineResourceState("Instance", models.ResourceStateSucceeded)
			machine.SetPublicIP(*instance.PublicIpAddress)
			machine.SetPrivateIP(*instance.PrivateIpAddress)

			// Wait for SSH connectivity
			if err := p.waitForSSHConnectivity(ctx, machine); err != nil {
				mu.Lock()
				machine.SetFailed(true)
				machine.SetMachineResourceState("SSH", models.ResourceStateFailed)
				mu.Unlock()
				return fmt.Errorf("SSH connectivity failed for VM %s: %w", machine.GetName(), err)
			}

			machine.SetMachineResourceState("SSH", models.ResourceStateSucceeded)
			machine.SetMachineResourceState("Network", models.ResourceStateSucceeded)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		l.Error("Failed to deploy VMs in parallel", zap.Error(err))
		return err
	}

	return nil
}

// CreateVM creates a new EC2 instance with the specified configuration
func (p *AWSProvider) CreateVM(
	ctx context.Context,
	machine models.Machiner,
) (*ec2.Instance, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	// Update status for creating instance
	m.UpdateStatus(models.NewDisplayStatusWithText(
		machine.GetName(),
		models.AWSResourceTypeInstance,
		models.ResourceStatePending,
		"Creating EC2 instance",
	))

	// Create the instance
	instance, err := p.EC2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String(machine.GetImageID()),
		InstanceType: types.InstanceType(machine.GetType().ResourceString),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		KeyName:      aws.String(m.Deployment.SSHKeyName),
		NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int32(0),
				AssociatePublicIpAddress: aws.Bool(true),
				DeleteOnTermination:      aws.Bool(true),
				SubnetId:                 aws.String(p.SubnetID),
				Groups:                   []string{p.SecurityGroupID},
			},
		},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(machine.GetName()),
					},
					{
						Key:   aws.String("Project"),
						Value: aws.String("andaime"),
					},
				},
			},
		},
	})

	if err != nil {
		l.Error("Failed to create EC2 instance",
			zap.String("machine", machine.GetName()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create EC2 instance: %w", err)
	}

	// Wait for instance to be running
	err = p.EC2Client.WaitUntilInstanceRunning(ctx, instance.InstanceId)
	if err != nil {
		l.Error("Failed waiting for instance to be running",
			zap.String("machine", machine.GetName()),
			zap.Error(err))
		return nil, fmt.Errorf("failed waiting for instance to be running: %w", err)
	}

	// Get instance details
	describeInstance, err := p.EC2Client.DescribeInstance(ctx, instance.InstanceId)
	if err != nil {
		l.Error("Failed to describe instance",
			zap.String("machine", machine.GetName()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to describe instance: %w", err)
	}

	return describeInstance, nil
}

// waitForSSHConnectivity polls a VM until SSH is available or max retries are reached
func (p *AWSProvider) waitForSSHConnectivity(ctx context.Context, machine models.Machiner) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	sshConfig := &sshutils.SSHConfig{
		User:               m.Deployment.SSHUser,
		Host:               machine.GetPublicIP(),
		Port:               m.Deployment.SSHPort,
		PrivateKeyMaterial: []byte(m.Deployment.SSHPrivateKeyMaterial),
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
