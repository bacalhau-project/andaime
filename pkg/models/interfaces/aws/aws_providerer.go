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
	GetEC2Client() (EC2Clienter, error)
	SetEC2Client(EC2Clienter)
	CreateDeployment(ctx context.Context, instanceType InstanceType) error
	ListDeployments(ctx context.Context) ([]*types.Instance, error)
	TerminateDeployment(ctx context.Context) error
	GetLatestUbuntuImage(ctx context.Context, region string) (*types.Image, error)
}
