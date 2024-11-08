package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type AWSProviderer interface {
	CreateDeployment(ctx context.Context) error
	ListDeployments(ctx context.Context) ([]string, error)
	TerminateDeployment(ctx context.Context) error
	GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error)
	GetEC2Client() (EC2Clienter, error)
	SetEC2Client(client EC2Clienter)
	Destroy(ctx context.Context) error
	GetVMExternalIP(ctx context.Context, instanceID string) (string, error)
	ValidateMachineType(ctx context.Context, location, instanceType string) (bool, error)
	CreateVPCAndSubnet(ctx context.Context) error
	StartResourcePolling(ctx context.Context) error
}

// CloudFormationAPI represents the AWS CloudFormation operations
type CloudFormationAPIer interface {
	GetTemplate(
		ctx context.Context,
		params *cloudformation.GetTemplateInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.GetTemplateOutput, error)
	DescribeStacks(
		ctx context.Context,
		params *cloudformation.DescribeStacksInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStacksOutput, error)
	DescribeStackEvents(
		ctx context.Context,
		params *cloudformation.DescribeStackEventsInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStackEventsOutput, error)
	CreateStack(
		ctx context.Context,
		params *cloudformation.CreateStackInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.CreateStackOutput, error)
	UpdateStack(
		ctx context.Context,
		params *cloudformation.UpdateStackInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.UpdateStackOutput, error)
	DeleteStack(
		ctx context.Context,
		params *cloudformation.DeleteStackInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.DeleteStackOutput, error)
	ListStacks(
		ctx context.Context,
		params *cloudformation.ListStacksInput,
		opts ...func(*cloudformation.Options),
	) (*cloudformation.ListStacksOutput, error)
}

// AWSInfraProvider defines the interface for AWS infrastructure operations
type AWSInfraProviderer interface {
	CreateVPC(ctx context.Context) error
	GetCloudFormationClient() CloudFormationAPIer
}
