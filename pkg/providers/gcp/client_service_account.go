package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
)

func (c *LiveGCPClient) setupServiceAccount(ctx context.Context, projectID string) error {
	sa, err := c.getOrCreateServiceAccount(ctx, projectID)
	if err != nil {
		return err
	}

	return c.ensureServiceAccountRoles(ctx, projectID, sa.Email)
}

func (c *LiveGCPClient) getOrCreateServiceAccount(
	ctx context.Context,
	projectID string,
) (*iam.ServiceAccount, error) {
	l := logger.Get()
	serviceAccountEmail := fmt.Sprintf(
		"%s@%s.iam.gserviceaccount.com",
		serviceAccountName,
		projectID,
	)

	sa, err := c.iamService.Projects.ServiceAccounts.Get(fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, serviceAccountEmail)).
		Context(ctx).
		Do()
	if err == nil {
		return sa, nil
	}

	if !isNotFoundError(err) {
		return nil, fmt.Errorf("failed to get service account: %v", err)
	}

	l.Infof("Creating new service account: %s", serviceAccountEmail)
	createReq := &iam.CreateServiceAccountRequest{
		AccountId: serviceAccountName,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: "Andaime Service Account",
		},
	}
	return c.iamService.Projects.ServiceAccounts.Create(fmt.Sprintf("projects/%s", projectID), createReq).
		Context(ctx).
		Do()
}

func (c *LiveGCPClient) ensureServiceAccountRoles(
	ctx context.Context,
	projectID, serviceAccountEmail string,
) error {
	requiredRoles := []string{
		"roles/compute.admin",
		"roles/storage.admin",
		"roles/iam.serviceAccountUser",
		"roles/cloudasset.owner",
		"roles/billing.projectManager",
	}

	for _, role := range requiredRoles {
		if err := c.grantRoleToServiceAccount(ctx, projectID, serviceAccountEmail, role); err != nil {
			return fmt.Errorf("failed to grant role %s: %v", role, err)
		}
	}

	return nil
}

func (c *LiveGCPClient) grantRoleToServiceAccount(
	ctx context.Context,
	projectID, serviceAccountEmail, role string,
) error {
	policy, err := c.resourceManagerService.Projects.GetIamPolicy(projectID, &cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("failed to get IAM policy: %v", err)
	}

	member := fmt.Sprintf("serviceAccount:%s", serviceAccountEmail)
	for _, binding := range policy.Bindings {
		if binding.Role == role {
			for _, m := range binding.Members {
				if m == member {
					return nil // Role already granted
				}
			}
			binding.Members = append(binding.Members, member)
			_, err := c.resourceManagerService.Projects.SetIamPolicy(projectID,
				&cloudresourcemanager.SetIamPolicyRequest{Policy: policy}).
				Context(ctx).
				Do()
			return err
		}
	}
	return err
}

func (c *LiveGCPClient) CreateServiceAccount(
	ctx context.Context,
	projectID string,
) (*iam.ServiceAccount, error) {
	l := logger.Get()
	l.Infof("Ensuring service account exists for project %s", projectID)

	serviceAccountName := fmt.Sprintf("%s-%s", projectID, "sa")

	if len(serviceAccountName) < 6 || len(serviceAccountName) > 30 {
		return nil, fmt.Errorf(
			"service account name is too long or too short, it should be between 6 and 30 characters",
		)
	}

	existingSA, err := c.iamService.Projects.ServiceAccounts.Get(fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", projectID, serviceAccountName, projectID)).
		Do()
	if err == nil {
		return existingSA, nil
	}

	if !isNotFoundError(err) {
		return nil, fmt.Errorf("failed to get existing service account: %w", err)
	}

	l.Infof("Service account %s does not exist, creating a new one", serviceAccountName)
	request := &iam.CreateServiceAccountRequest{
		AccountId: serviceAccountName,
	}
	return c.iamService.Projects.ServiceAccounts.Create("projects/"+projectID, request).Do()
}
