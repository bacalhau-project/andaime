package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type InstanceType string

const (
	EC2Instance  InstanceType = "EC2"
	SpotInstance InstanceType = "Spot"
)

type AWSProviderer interface {
	CreateDeployment(ctx context.Context, instanceType InstanceType) error
	ListDeployments(ctx context.Context) ([]*types.Instance, error)
	TerminateDeployment(ctx context.Context) error
	GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error)
	GetEC2Client() (EC2Clienter, error)
	SetEC2Client(client EC2Clienter)
	Destroy(ctx context.Context) error
	GetVMExternalIP(ctx context.Context, instanceID string) (string, error)
	ValidateMachineType(ctx context.Context, location, instanceType string) (bool, error)
}
