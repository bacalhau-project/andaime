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
	err := p.EnsureProject(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure project exists: %w", err)
	}

	// Enable required APIs
	if err := p.EnableRequiredAPIs(ctx); err != nil {
		return fmt.Errorf("failed to enable required APIs: %w", err)
	}

	var eg errgroup.Group
	eg.Go(func() error {
		return p.CreateVPCNetwork(ctx, "default")
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to create VPC rules: %v", err)
	}

	eg = errgroup.Group{}
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

			publicIP, privateIP, err := p.CreateVM(
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

			machine.SetPublicIP(publicIP)
			machine.SetPrivateIP(privateIP)

			sshConfig, err := sshutils.NewSSHConfigFunc(
				machine.GetPublicIP(),
				machine.GetSSHPort(),
				machine.GetSSHUser(),
				machine.GetSSHPrivateKeyPath(),
			)
			if err != nil {
				return fmt.Errorf("failed to create SSH config: %w", err)
			}
			machine.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateUpdating)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Provisioning SSH",
			))

			if err := sshConfig.WaitForSSH(ctx, sshutils.SSHRetryAttempts, sshutils.GetAggregateSSHTimeout()); err != nil {
				l.Errorf("Failed to provision SSH: %v", err)
				machine.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateFailed)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.GetName(),
					models.GCPResourceTypeInstance,
					models.ResourceStateFailed,
					"SSH Provisioning Failed",
				))

				return fmt.Errorf("failed to provision SSH: %w", err)
			}

			machine.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateSucceeded)
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

			l.Infof("Instance %s created successfully", machine.GetName())

			return nil
		})
	}

	if err := instanceEg.Wait(); err != nil {
		return fmt.Errorf("failed to create instances: %v", err)
	}

	return nil
}
