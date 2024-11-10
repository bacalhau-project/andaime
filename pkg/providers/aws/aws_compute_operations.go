package awsprovider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"golang.org/x/sync/errgroup"
)

const (
	SSHRetryInterval = 10 * time.Second
	MaxRetries       = 30
)

// DeployVMsInParallel deploys multiple VMs in parallel and waits for SSH connectivity
func (p *AWSProvider) DeployVMsInParallel(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	var g errgroup.Group
	var mu sync.Mutex

	// Deploy all VMs in parallel
	for _, machine := range m.Deployment.GetMachines() {
		machine := machine // Create local copy for goroutine
		g.Go(func() error {
			// Create and configure the VM
			if err := p.CreateAndConfigureVM(ctx, machine); err != nil {
				mu.Lock()
				machine.SetFailed(true)
				mu.Unlock()
				return fmt.Errorf("failed to create VM %s: %w", machine.GetName(), err)
			}

			// Wait for SSH connectivity
			if err := p.waitForSSHConnectivity(ctx, machine); err != nil {
				mu.Lock()
				machine.SetFailed(true)
				mu.Unlock()
				return fmt.Errorf("SSH connectivity failed for VM %s: %w", machine.GetName(), err)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		l.Error("Failed to deploy VMs in parallel: ", err)
		return err
	}

	return nil
}

// waitForSSHConnectivity polls a VM until SSH is available or max retries are reached
func (p *AWSProvider) waitForSSHConnectivity(ctx context.Context, machine models.Machiner) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	sshConfig := &sshutils.SSHConfig{
		User:          p.SSHUser,
		Host:          machine.GetPublicIP(),
		Port:          p.SSHPort,
		PrivateKeyPath: p.SSHPrivateKeyPath,
	}

	for i := 0; i < MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := sshConfig.WaitForSSH(ctx)
			if err == nil {
				l.Infof("SSH connectivity established for VM %s", machine.GetName())
				machine.SetMachineResourceState("SSH", models.ResourceStateSucceeded)
				return nil
			}

			l.Debugf("Waiting for SSH on VM %s (attempt %d/%d): %v", 
				machine.GetName(), i+1, MaxRetries, err)
			machine.SetMachineResourceState("SSH", models.ResourceStatePending)

			// Update the model with current status
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.AWSResourceTypeInstance,
				models.ResourceStatePending,
				fmt.Sprintf("Waiting for SSH connectivity (attempt %d/%d)", i+1, MaxRetries),
			))

			time.Sleep(SSHRetryInterval)
		}
	}

	return fmt.Errorf("SSH connectivity timeout after %d attempts for VM %s", MaxRetries, machine.GetName())
}
