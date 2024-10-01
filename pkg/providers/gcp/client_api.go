package gcp

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/serviceusage/apiv1/serviceusagepb"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func (c *LiveGCPClient) enableRequiredAPIs(ctx context.Context, projectID string) error {
	requiredAPIs := []string{
		"compute.googleapis.com",
		"cloudasset.googleapis.com",
		"cloudresourcemanager.googleapis.com",
		"iam.googleapis.com",
		"storage-api.googleapis.com",
		"storage-component.googleapis.com",
	}

	for _, api := range requiredAPIs {
		if err := c.EnableAPI(ctx, projectID, api); err != nil {
			return fmt.Errorf("failed to enable API %s: %v", api, err)
		}
	}

	return nil
}

func (c *LiveGCPClient) EnableAPI(ctx context.Context, projectID, apiName string) error {
	l := logger.Get()
	l.Infof("Enabling API %s for project %s", apiName, projectID)

	serviceName := fmt.Sprintf("projects/%s/services/%s", projectID, apiName)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = maxBackOffTime

	return backoff.Retry(func() error {
		_, err := c.serviceUsageClient.EnableService(ctx, &serviceusagepb.EnableServiceRequest{
			Name: serviceName,
		})
		if err != nil {
			if status.Code(err) == codes.PermissionDenied {
				return backoff.Permanent(err)
			}
			return err
		}
		return nil
	}, b)
}

func (c *LiveGCPClient) IsAPIEnabled(ctx context.Context, projectID, apiName string) (bool, error) {
	l := logger.Get()
	l.Infof("Checking if API %s is enabled for project %s", apiName, projectID)

	if projectID == "" {
		return false, fmt.Errorf("project ID is empty")
	}

	serviceName := fmt.Sprintf("projects/%s/services/%s", projectID, apiName)
	service, err := c.serviceUsageClient.GetService(ctx, &serviceusagepb.GetServiceRequest{
		Name: serviceName,
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if API is enabled: %v", err)
	}

	return service.State == serviceusagepb.State_ENABLED, nil
}
