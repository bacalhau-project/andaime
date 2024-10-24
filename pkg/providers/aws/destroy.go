package awsprovider

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

// DestroyResources deletes the specified AWS VPC and associated resources
func (p *AWSProvider) DestroyResources(ctx context.Context, vpcID string) error {
	l := logger.Get()
	l.Infof("Starting destruction of AWS deployment (VPC ID: %s)", vpcID)

	// Call the Destroy method we implemented
	err := p.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy AWS resources: %w", err)
	}

	l.Info("AWS resources successfully destroyed")
	return nil
}
