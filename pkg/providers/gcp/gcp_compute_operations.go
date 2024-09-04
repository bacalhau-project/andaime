package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"google.golang.org/api/compute/v1"
)

func (p *GCPProvider) waitForZoneOperation(
	ctx context.Context,
	computeService *compute.Service,
	zone, operationName string,
) error {
	m := display.GetGlobalModelFunc()

	for {
		operation, err := computeService.ZoneOperations.Get(m.Deployment.ProjectID, zone, operationName).
			Do()
		if err != nil {
			return fmt.Errorf("failed to get operation: %w", err)
		}

		if operation.Status == "DONE" {
			if operation.Error != nil {
				return fmt.Errorf("operation failed: %v", operation.Error.Errors)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			// Continue waiting
		}
	}
}
