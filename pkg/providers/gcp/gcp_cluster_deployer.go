package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"golang.org/x/sync/errgroup"
)

// CreateResources implements the ClusterDeployer interface for GCP
func (p *GCPProvider) CreateResources(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	// Create the project if it doesn't exist
	createdProjectID, err := p.EnsureProject(ctx, m.Deployment.GCP.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to ensure project exists: %w", err)
	}

	m.Deployment.ProjectID = createdProjectID
	m.Deployment.GCP.ProjectID = createdProjectID

	// Enable required APIs
	if err := p.EnableRequiredAPIs(ctx); err != nil {
		return fmt.Errorf("failed to enable required APIs: %w", err)
	}

	var eg errgroup.Group
	// Create firewall rules for the project
	eg.Go(func() error {
		return p.CreateFirewallRules(ctx, "default")
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to create firewall rules: %v", err)
	}

	var instanceEg errgroup.Group
	for _, machine := range m.Deployment.Machines {
		instanceEg.Go(func() error {
			l.Infof("Creating instance %s in zone %s", machine.GetName(), machine.GetLocation())
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Creating VM",
			))

			instance, err := p.Client.CreateComputeInstance(
				ctx,
				machine.GetName(),
			)
			if err != nil {
				l.Errorf("Failed to create instance %s: %v", machine.GetName(), err)
				machine.SetMachineResourceState(
					models.GCPResourceTypeInstance.ResourceString,
					models.ResourceStateFailed,
				)
				return err
			}

			machine.SetPublicIP(*instance.NetworkInterfaces[0].AccessConfigs[0].NatIP)
			machine.SetPrivateIP(*instance.NetworkInterfaces[0].NetworkIP)

			sshConfig, err := sshutils.NewSSHConfigFunc(
				machine.GetPublicIP(),
				machine.GetSSHPort(),
				machine.GetSSHUser(),
				machine.GetSSHPrivateKeyPath(),
			)
			if err != nil {
				return fmt.Errorf("failed to create SSH config: %w", err)
			}
			machine.SetServiceState("SSH", models.ServiceStateUpdating)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Provisioning SSH",
			))

			if err := sshConfig.WaitForSSH(ctx, sshutils.SSHRetryAttempts, sshutils.GetAggregateSSHTimeout()); err != nil {
				l.Errorf("Failed to provision SSH: %v", err)
				machine.SetServiceState("SSH", models.ServiceStateFailed)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.GetName(),
					models.GCPResourceTypeInstance,
					models.ResourceStateFailed,
					"SSH Provisioning Failed",
				))

				return fmt.Errorf("failed to provision SSH: %w", err)
			}

			machine.SetServiceState("SSH", models.ServiceStateSucceeded)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeInstance,
				models.ResourceStateRunning,
				"SSH Provisioned",
			))

			machine.SetMachineResourceState(
				models.GCPResourceTypeInstance.ResourceString,
				models.ResourceStateRunning,
			)

			if len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
				machine.SetPublicIP(*instance.NetworkInterfaces[0].AccessConfigs[0].NatIP)
			} else {
				return fmt.Errorf("no access configs found for instance %s - could not get public IP", machine.GetName())
			}

			l.Infof("Instance %s created successfully", machine.GetName())

			// Create or ensure Cloud Storage bucket
			// bucketName := fmt.Sprintf("%s-storage", m.Deployment.ProjectID)
			// l.Infof("Ensuring Cloud Storage bucket: %s\n", bucketName)
			// if err := p.EnsureStorageBucket(ctx, machine.Location, bucketName); err != nil {
			// 	return fmt.Errorf("failed to ensure storage bucket: %v", err)
			// }

			return nil
		})
	}

	if err := instanceEg.Wait(); err != nil {
		return fmt.Errorf("failed to create instances: %v", err)
	}

	return nil
}
