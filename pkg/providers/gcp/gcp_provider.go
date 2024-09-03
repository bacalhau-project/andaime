package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"google.golang.org/api/compute/v1"
)

func (p *GCPProvider) CreateComputeInstance(instanceName, machineType, zone string) error {
	// TODO: Consider moving common instance creation logic to a shared function
	m := display.GetGlobalModelFunc()
	computeService, err := compute.NewService(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create Compute Engine service: %v", err)
	}

	tags := GenerateTags(m.Deployment.ProjectID, m.Deployment.UniqueID)

	instance := &compute.Instance{
		Name:        instanceName,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType),
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: "global/networks/default",
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
			},
		},
		Labels: tags,
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: "projects/debian-cloud/global/images/debian-10-buster-v20230321",
					DiskSizeGb:  10,
				},
			},
		},
	}

	op, err := computeService.Instances.Insert(m.Deployment.ProjectID, zone, instance).Do()
	if err != nil {
		return fmt.Errorf("failed to create Compute Engine instance: %v", err)
	}

	// Wait for the instance creation to complete
	err = p.waitForZoneOperation(computeService, zone, op.Name)
	if err != nil {
		return fmt.Errorf("failed to wait for instance creation: %v", err)
	}

	return nil
}

func (p *GCPProvider) waitForZoneOperation(
	computeService *compute.Service,
	zone, operationName string,
) error {
	m := display.GetGlobalModelFunc()
	for {
		op, err := computeService.ZoneOperations.Get(m.Deployment.ProjectID, zone, operationName).
			Do()
		if err != nil {
			return err
		}
		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("operation failed: %v", op.Error.Errors[0].Message)
			}
			return nil
		}
		time.Sleep(5 * time.Second)
	}
}
