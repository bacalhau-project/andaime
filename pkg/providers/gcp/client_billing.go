package gcp

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/billing/apiv1/billingpb"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/iterator"
)

func (c *LiveGCPClient) SetBillingAccount(
	ctx context.Context,
	projectID string,
	billingAccountID string,
) error {
	l := logger.Get()
	l.Infof("Setting billing account %s for project %s", billingAccountID, projectID)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = maxBackOffTime

	return backoff.Retry(func() error {
		err := c.EnsureServiceAccount(ctx, projectID, serviceAccountName)
		if err != nil {
			l.Warnf("Failed to ensure service account: %v. Retrying...", err)
			return err
		}

		req := &billingpb.UpdateProjectBillingInfoRequest{
			Name: fmt.Sprintf("projects/%s", projectID),
			ProjectBillingInfo: &billingpb.ProjectBillingInfo{
				BillingAccountName: getBillingAccountName(billingAccountID),
				BillingEnabled:     true,
			},
		}

		_, err = c.billingClient.UpdateProjectBillingInfo(ctx, req)
		if err != nil {
			if strings.Contains(err.Error(), "service account") {
				l.Warnf("Service account issue when setting billing account: %v. Retrying...", err)
				return err // Retry on service account issues
			}
			return backoff.Permanent(fmt.Errorf("failed to update billing info: %v", err))
		}

		l.Infof("Successfully set billing account %s for project %s using service account %s",
			billingAccountID, projectID, serviceAccountName)
		return nil
	}, b)
}

func (c *LiveGCPClient) ListBillingAccounts(ctx context.Context) ([]string, error) {
	l := logger.Get()
	l.Debug("Listing billing accounts")

	req := &billingpb.ListBillingAccountsRequest{}
	it := c.billingClient.ListBillingAccounts(ctx, req)

	var billingAccounts []string
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			l.Errorf("Failed to list billing accounts: %v", err)
			return nil, fmt.Errorf("failed to list billing accounts: %v", err)
		}
		billingAccounts = append(billingAccounts, resp.Name)
	}

	return billingAccounts, nil
}

func getBillingAccountName(billingAccountID string) string {
	if strings.HasPrefix(billingAccountID, "billingAccounts/") {
		return billingAccountID
	}
	return fmt.Sprintf("billingAccounts/%s", billingAccountID)
}
