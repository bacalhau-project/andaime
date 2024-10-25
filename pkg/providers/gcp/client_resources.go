package gcp

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"google.golang.org/api/iterator"
)

func (c *LiveGCPClient) StartResourcePolling(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.Quitting {
				l.Debug("Quitting detected, stopping resource polling")
				return nil
			}

			if m.Deployment.GCP.ProjectID == "" {
				continue
			}

			resources, err := c.ListAllAssetsInProject(ctx, m.Deployment.GCP.ProjectID)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
				continue
			}

			l.Debugf("Poll: Found %d resources", len(resources))

			allResourcesProvisioned := true
			for _, resource := range resources {
				if err := c.UpdateResourceState(resource.GetName(), resource.GetAssetType(), models.ResourceStateSucceeded); err != nil {
					l.Errorf("Failed to update resource state: %v", err)
					allResourcesProvisioned = false
				}
			}

			if allResourcesProvisioned && c.allMachinesComplete(m) {
				l.Debug(
					"All resources provisioned and machines completed, stopping resource polling",
				)
				return nil
			}

		case <-ctx.Done():
			l.Debug("Context cancelled, exiting resource polling")
			return ctx.Err()
		}
	}
}

func (c *LiveGCPClient) allMachinesComplete(m *display.DisplayModel) bool {
	for _, machine := range m.Deployment.GetMachines() {
		if !machine.IsComplete() {
			return false
		}
	}
	return true
}

func (c *LiveGCPClient) ListAllAssetsInProject(
	ctx context.Context,
	projectID string,
) ([]*assetpb.Asset, error) {
	resources := []*assetpb.Asset{}
	l := logger.Get()

	assetTypes := []string{
		"compute.googleapis.com/instance",
		"compute.googleapis.com/disk",
		"compute.googleapis.com/image",
		"compute.googleapis.com/network",
		"compute.googleapis.com/firewall",
	}

	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("projects/%s", projectID),
		AssetTypes: assetTypes,
	}

	it := c.assetClient.SearchAllResources(ctx, req)
	for {
		resource, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %v", err)
		}

		resourceAsset := &assetpb.Asset{
			Name:       resource.GetName(),
			UpdateTime: resource.GetUpdateTime(),
			AssetType:  resource.GetAssetType(),
		}
		resources = append(resources, resourceAsset)
	}

	if rand.Int31n(100) < 10 {
		l.Debugf("Found %d resources", len(resources))
	}

	return resources, nil
}

func (c *LiveGCPClient) UpdateResourceState(
	resourceName, resourceType string,
	state models.MachineResourceState,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	foundResource := false
	for _, machine := range m.Deployment.GetMachines() {
		if strings.Contains(strings.ToLower(resourceName), strings.ToLower(machine.GetName())) {
			if machine.GetMachineResourceState(resourceType) < state {
				machine.SetMachineResourceState(resourceType, state)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.GetName(),
					models.GetGCPResourceType(resourceType),
					state,
					resourceType+" deployed.",
				))
			}
			return nil
		} else if strings.Contains(strings.ToLower(resourceName), "/global/") {
			foundResource = true
			if machine.GetMachineResourceState(resourceType) < state {
				machine.SetMachineResourceState(resourceType, state)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.GetName(),
					models.GetGCPResourceType(resourceType),
					state,
					resourceType+" deployed.",
				))
			}
		}
	}

	if foundResource {
		return nil
	}

	return fmt.Errorf("resource %s not found in any machine", resourceName)
}

func (c *LiveGCPClient) EnsureFirewallRules(ctx context.Context, networkName string) error {
	l := logger.Get()
	l.Debugf("Ensuring firewall rules for network %s", networkName)

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	network, err := c.getOrCreateNetwork(ctx, m.Deployment.GetProjectID(), networkName)
	if err != nil {
		return fmt.Errorf("failed to get or create network: %v", err)
	}

	for i, port := range m.Deployment.AllowedPorts {
		firewallRuleName := fmt.Sprintf("allow-%d", port)
		firewallRule := &computepb.Firewall{
			Name:    to.Ptr(firewallRuleName),
			Network: network.SelfLink,
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: to.Ptr("tcp"),
					Ports:      []string{strconv.Itoa(port)},
				},
			},
			Direction: to.Ptr("INGRESS"),
			Priority:  to.Ptr(int32(1000 + i)), //nolint:gosec,mnd
		}

		req := &computepb.InsertFirewallRequest{
			Project:          m.Deployment.GetProjectID(),
			FirewallResource: firewallRule,
		}

		_, err = c.firewallsClient.Insert(ctx, req)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				l.Debugf("Firewall rule %s already exists, skipping creation", firewallRuleName)
				continue
			}
			return fmt.Errorf("failed to create firewall rule: %v", err)
		}
	}

	return nil
}
