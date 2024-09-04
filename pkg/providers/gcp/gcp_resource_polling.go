package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"google.golang.org/api/compute/v1"
)

func (p *GCPProvider) PollAndUpdateResources(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	computeService, err := compute.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create compute service: %w", err)
	}

	for _, machine := range m.Deployment.Machines {
		instance, err := computeService.Instances.Get(m.Deployment.ProjectID, machine.Location, machine.Name).
			Do()
		if err != nil {
			l.Errorf("Failed to get instance %s: %v", machine.Name, err)
			continue
		}

		updateResourceState(machine, instance)
	}

	return nil
}

func updateResourceState(machine *models.Machine, instance *compute.Instance) {
	switch instance.Status {
	case "PROVISIONING", "STAGING":
		machine.SetResourceState(
			models.GCPResourceTypeInstance.ResourceString,
			models.ResourceStatePending,
		)
	case "RUNNING":
		machine.SetResourceState(
			models.GCPResourceTypeInstance.ResourceString,
			models.ResourceStateRunning,
		)
		machine.PublicIP = instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
		machine.PrivateIP = instance.NetworkInterfaces[0].NetworkIP
	case "STOPPING", "SUSPENDING":
		machine.SetResourceState(
			models.GCPResourceTypeInstance.ResourceString,
			models.ResourceStateStopping,
		)
	case "TERMINATED", "SUSPENDED":
		machine.SetResourceState(
			models.GCPResourceTypeInstance.ResourceString,
			models.ResourceStateTerminated,
		)
	default:
		machine.SetResourceState(
			models.GCPResourceTypeInstance.ResourceString,
			models.ResourceStateUnknown,
		)
	}
}

func ConvertGCPResourceState(state string) models.GCPResourceState {
	switch state {
	case "PROVISIONING", "STAGING":
		return models.GCPResourceStateProvisioning
	case "RUNNING":
		return models.GCPResourceStateRunning
	case "STOPPING", "SUSPENDING":
		return models.GCPResourceStateStopping
	case "TERMINATED", "SUSPENDED":
		return models.GCPResourceStateTerminated
	}

	return models.GCPResourceStateUnknown
}
