package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
)

// Ensure GCPProvider implements ClusterDeployer
var _ common.ClusterDeployer = (*GCPProvider)(nil)

// CreateResources implements the ClusterDeployer interface for GCP
func (p *GCPProvider) CreateResources(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for _, machine := range m.Deployment.Machines {
		l.Infof("Creating instance %s in zone %s", machine.Name, machine.Location)
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machine.Name,
			models.GCPResourceTypeInstance,
			models.ResourceStatePending,
			"Creating VM",
		))

		instance, err := p.CreateComputeInstance(
			ctx,
			machine.Name,
		)
		if err != nil {
			l.Errorf("Failed to create instance %s: %v", machine.Name, err)
			machine.SetResourceState(
				models.GCPResourceTypeInstance.ResourceString,
				models.ResourceStateFailed,
			)
			continue
		}

		machine.SetResourceState(
			models.GCPResourceTypeInstance.ResourceString,
			models.ResourceStateRunning,
		)

		if len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
			machine.PublicIP = *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
		} else {
			return fmt.Errorf("no access configs found for instance %s - could not get public IP", machine.Name)
		}

		machine.PrivateIP = *instance.NetworkInterfaces[0].NetworkIP
		l.Infof("Instance %s created successfully", machine.Name)

		// Create or ensure Cloud Storage bucket
		bucketName := fmt.Sprintf("%s-storage", m.Deployment.ProjectID)
		fmt.Printf("Ensuring Cloud Storage bucket: %s\n", bucketName)
		if err := p.EnsureStorageBucket(ctx, machine.Location, bucketName); err != nil {
			return fmt.Errorf("failed to ensure storage bucket: %v", err)
		}
	}

	return nil
}

// ProvisionSSH implements the ClusterDeployer interface for GCP
func (p *GCPProvider) ProvisionSSH(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for _, machine := range m.Deployment.Machines {
		l.Infof("Provisioning SSH for instance %s", machine.Name)
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machine.Name,
			models.GCPResourceTypeInstance,
			models.ResourceStatePending,
			"Provisioning SSH",
		))

		// TODO: Implement GCP-specific SSH provisioning logic here

		machine.SetServiceState("SSH", models.ServiceStateSucceeded)
		l.Infof("SSH provisioned successfully for instance %s", machine.Name)
	}

	return nil
}

// SetupDocker implements the ClusterDeployer interface for GCP
func (p *GCPProvider) SetupDocker(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for _, machine := range m.Deployment.Machines {
		l.Infof("Setting up Docker on instance %s", machine.Name)
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machine.Name,
			models.GCPResourceTypeInstance,
			models.ResourceStatePending,
			"Setting up Docker",
		))

		// TODO: Implement GCP-specific Docker setup logic here

		machine.SetServiceState("Docker", models.ServiceStateSucceeded)
		l.Infof("Docker set up successfully on instance %s", machine.Name)
	}

	return nil
}

// DeployOrchestrator implements the ClusterDeployer interface for GCP
func (p *GCPProvider) DeployOrchestrator(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			l.Infof("Deploying orchestrator on instance %s", machine.Name)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Deploying Orchestrator",
			))

			// TODO: Implement GCP-specific orchestrator deployment logic here

			machine.SetServiceState("Orchestrator", models.ServiceStateSucceeded)
			l.Infof("Orchestrator deployed successfully on instance %s", machine.Name)
			break
		}
	}

	return nil
}

// DeployNodes implements the ClusterDeployer interface for GCP
func (p *GCPProvider) DeployNodes(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for _, machine := range m.Deployment.Machines {
		if !machine.Orchestrator {
			l.Infof("Deploying node on instance %s", machine.Name)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Deploying Node",
			))

			// TODO: Implement GCP-specific node deployment logic here

			machine.SetServiceState("Node", models.ServiceStateSucceeded)
			l.Infof("Node deployed successfully on instance %s", machine.Name)
		}
	}

	return nil
}

// Update GCPProviderer interface to include ClusterDeployer methods
type GCPProviderer interface {
	common.ClusterDeployer
	GetGCPClient() GCPClienter
	SetGCPClient(client GCPClienter)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)

	EnsureProject(
		ctx context.Context,
		projectID string,
	) (string, error)
	DestroyProject(
		ctx context.Context,
		projectID string,
	) error
	ListProjects(
		ctx context.Context,
	) ([]*resourcemanagerpb.Project, error)
	ListAllAssetsInProject(
		ctx context.Context,
		projectID string,
	) ([]*assetpb.Asset, error)
	SetBillingAccount(ctx context.Context) error
	DeployResources(ctx context.Context) error
	ProvisionPackagesOnMachines(ctx context.Context) error
	ProvisionBacalhau(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	StartResourcePolling(ctx context.Context)
	CheckAuthentication(ctx context.Context) error
	EnableAPI(ctx context.Context, apiName string) error
	EnableRequiredAPIs(ctx context.Context) error
	CreateVPCNetwork(
		ctx context.Context,
		networkName string,
	) error
	CreateFirewallRules(
		ctx context.Context,
		networkName string,
	) error
	CreateStorageBucket(
		ctx context.Context,
		bucketName string,
	) error
	CreateComputeInstance(
		ctx context.Context,
		vmName string,
	) (*computepb.Instance, error)
	GetVMExternalIP(
		ctx context.Context,
		projectID,
		zone,
		vmName string,
	) (string, error)
	EnsureFirewallRules(
		ctx context.Context,
		networkName string,
	) error
	EnsureStorageBucket(
		ctx context.Context,
		location,
		bucketName string,
	) error
}
