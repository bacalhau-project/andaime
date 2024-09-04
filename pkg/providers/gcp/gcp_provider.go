package gcp

import (
	"context"

	"cloud.google.com/go/compute/apiv1/computepb"
)

func (p *GCPProvider) CreateComputeInstance(
	ctx context.Context,
	instanceName string,
) (*computepb.Instance, error) {
	instance, err := p.GetGCPClient().CreateComputeInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	return instance, nil
}
