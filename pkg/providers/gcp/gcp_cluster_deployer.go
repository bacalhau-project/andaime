package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
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
			l.Infof("Creating instance %s in zone %s", machine.Name, machine.Location)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Creating VM",
			))

			instance, err := p.Client.CreateComputeInstance(
				ctx,
				machine.Name,
			)
			if err != nil {
				l.Errorf("Failed to create instance %s: %v", machine.Name, err)
				machine.SetResourceState(
					models.GCPResourceTypeInstance.ResourceString,
					models.ResourceStateFailed,
				)
				return err
			}

			machine.PublicIP = *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
			machine.PrivateIP = *instance.NetworkInterfaces[0].NetworkIP

			sshConfig, err := sshutils.NewSSHConfigFunc(
				machine.PublicIP,
				machine.SSHPort,
				machine.SSHUser,
				machine.SSHPrivateKeyPath,
			)
			if err != nil {
				return fmt.Errorf("failed to create SSH config: %w", err)
			}
			machine.SetServiceState("SSH", models.ServiceStateUpdating)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeInstance,
				models.ResourceStatePending,
				"Provisioning SSH",
			))

			if err := sshConfig.WaitForSSH(ctx, sshutils.SSHRetryAttempts, sshutils.GetAggregateSSHTimeout()); err != nil {
				l.Errorf("Failed to provision SSH: %v", err)
				machine.SetServiceState("SSH", models.ServiceStateFailed)
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.Name,
					models.GCPResourceTypeInstance,
					models.ResourceStateFailed,
					"SSH Provisioning Failed",
				))

				return fmt.Errorf("failed to provision SSH: %w", err)
			}

			machine.SetServiceState("SSH", models.ServiceStateSucceeded)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.Name,
				models.GCPResourceTypeInstance,
				models.ResourceStateRunning,
				"SSH Provisioned",
			))

			machine.SetResourceState(
				models.GCPResourceTypeInstance.ResourceString,
				models.ResourceStateRunning,
			)

			if len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
				machine.PublicIP = *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
			} else {
				return fmt.Errorf("no access configs found for instance %s - could not get public IP", machine.Name)
			}

			l.Infof("Instance %s created successfully", machine.Name)

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

// Update GCPProviderer interface to include ClusterDeployer methods
type GCPProviderer interface {
	GetGCPClient() GCPClienter
	SetGCPClient(client GCPClienter)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)
	GetClusterDeployer() *common.ClusterDeployer
	SetClusterDeployer(deployer *common.ClusterDeployer)

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

	CreateResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error

	StartResourcePolling(ctx context.Context)
	CheckAuthentication(ctx context.Context) error
	EnableAPI(ctx context.Context, apiName string) error
	EnableRequiredAPIs(ctx context.Context) error

	// CreateVPCNetwork(
	// 	ctx context.Context,
	// 	networkName string,
	// ) error
	// CreateFirewallRules(
	// 	ctx context.Context,
	// 	networkName string,
	// ) error
	// CreateStorageBucket(
	// 	ctx context.Context,
	// 	bucketName string,
	// ) error
	// CreateComputeInstance(
	// 	ctx context.Context,
	// 	vmName string,
	// ) (*computepb.Instance, error)
	// GetVMExternalIP(
	// 	ctx context.Context,
	// 	projectID,
	// 	zone,
	// 	vmName string,
	// ) (string, error)
	// EnsureFirewallRules(
	// 	ctx context.Context,
	// 	networkName string,
	// ) error
	// EnsureStorageBucket(
	// 	ctx context.Context,
	// 	location,
	// 	bucketName string,
	// ) error
}
