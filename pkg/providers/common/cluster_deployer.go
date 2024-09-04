package common

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
)

// ClusterDeployer defines the interface for deploying a cluster
type ClusterDeployer interface {
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)

	// CreateResources creates all necessary cloud resources for the cluster
	CreateResources(ctx context.Context) error

	// ProvisionSSH sets up SSH access to the cluster nodes
	ProvisionSSH(ctx context.Context) error

	// SetupDocker installs and configures Docker on the cluster nodes
	SetupDocker(ctx context.Context) error

	// DeployOrchestrator deploys the Bacalhau orchestrator
	DeployOrchestrator(ctx context.Context) error

	// DeployNodes deploys the Bacalhau worker nodes
	DeployNodes(ctx context.Context) error
}
