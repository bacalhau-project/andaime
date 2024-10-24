package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"golang.org/x/sync/errgroup"
)

// IPAllocationConfig holds settings for IP allocation attempts
type IPAllocationConfig struct {
	MaxRetries    int
	RetryInterval time.Duration
	Timeout       time.Duration
}

// DefaultIPAllocationConfig provides reasonable defaults
var DefaultIPAllocationConfig = IPAllocationConfig{
	MaxRetries:    3,               //nolint:mnd
	RetryInterval: 5 * time.Second, //nolint:mnd
	Timeout:       2 * time.Minute, //nolint:mnd
}

// CreateResources implements the ClusterDeployer interface for GCP
func (p *GCPProvider) CreateResources(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if err := p.initializeProject(ctx); err != nil {
		return err
	}

	if err := p.setupNetworking(ctx); err != nil {
		return err
	}

	// Find orchestrator machine
	orchestratorMachine := findOrchestratorMachine(m.Deployment.Machines)
	if orchestratorMachine == nil {
		return fmt.Errorf("no orchestrator machine configured")
	}

	// Create orchestrator first - this must succeed
	if err := p.provisionMachine(ctx, orchestratorMachine, true); err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Create worker nodes in parallel
	var instanceEg errgroup.Group
	for _, machine := range m.Deployment.Machines {
		if machine.IsOrchestrator() {
			continue
		}
		machine := machine // Create new variable for goroutine
		instanceEg.Go(func() error {
			// Worker failures are logged but don't stop deployment
			if err := p.provisionMachine(ctx, machine, false); err != nil {
				l.Errorf("Failed to provision worker %s: %v", machine.GetName(), err)
				machine.SetFailed(true)
			}
			return nil
		})
	}

	return instanceEg.Wait()
}

// Helper functions to break down the implementation

func (p *GCPProvider) initializeProject(ctx context.Context) error {
	if err := p.EnsureProject(ctx); err != nil {
		return fmt.Errorf("failed to ensure project exists: %w", err)
	}

	if err := p.EnableRequiredAPIs(ctx); err != nil {
		return fmt.Errorf("failed to enable required APIs: %w", err)
	}

	return nil
}

func (p *GCPProvider) setupNetworking(ctx context.Context) error {
	var eg errgroup.Group

	eg.Go(func() error {
		return p.CreateVPCNetwork(ctx, "default")
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to create VPC rules: %w", err)
	}

	eg = errgroup.Group{}
	eg.Go(func() error {
		return p.CreateFirewallRules(ctx, "default")
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to create firewall rules: %w", err)
	}

	return nil
}

func findOrchestratorMachine(machines map[string]models.Machiner) models.Machiner {
	for _, machine := range machines {
		if machine.IsOrchestrator() {
			return machine
		}
	}
	return nil
}

func (p *GCPProvider) provisionMachine(
	ctx context.Context,
	machine models.Machiner,
	isOrchestrator bool,
) error {
	l := logger.Get()

	// Create VM
	l.Infof("Creating instance %s in zone %s", machine.GetName(), machine.GetLocation())
	if err := p.CreateAndConfigureVM(ctx, machine); err != nil {
		return fmt.Errorf("failed to create and configure VM: %w", err)
	}

	// Setup SSH
	if err := p.setupSSH(ctx, machine); err != nil {
		return fmt.Errorf("failed to setup SSH: %w", err)
	}

	machine.SetMachineResourceState(
		models.GCPResourceTypeInstance.ResourceString,
		models.ResourceStateRunning,
	)

	l.Infof("Instance %s created successfully", machine.GetName())
	return nil
}

func (p *GCPProvider) setupSSH(ctx context.Context, machine models.Machiner) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

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

	return nil
}
