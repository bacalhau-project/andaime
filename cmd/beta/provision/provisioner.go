package provision

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

const (
	SSHTimeOut = 60 * time.Second
)

// Provisioner handles the node provisioning process
type Provisioner struct {
	sshConfig sshutils.SSHConfiger
	nodeType  NodeType
	config    *NodeConfig
	machine   models.Machiner
}

// NewProvisioner creates a new Provisioner instance
func NewProvisioner(config *NodeConfig) (*Provisioner, error) {
	sshConfig, err := sshutils.NewSSHConfig(
		config.IPAddress,
		22, // Default SSH port
		config.Username,
		config.PrivateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH config: %w", err)
	}

	// Create a minimal machine instance just for software installation
	machine, err := models.NewMachine(models.DeploymentTypeGeneric, "", "", 0, models.CloudSpecificInfo{})
	if err != nil {
		return nil, fmt.Errorf("failed to create machine: %w", err)
	}

	// Set only the required SSH and node configuration
	machine.SetSSHUser(config.Username)
	machine.SetSSHPrivateKeyPath(config.PrivateKey)
	machine.SetSSHPort(22)
	machine.SetPublicIP(config.IPAddress)
	machine.SetNodeType(string(config.NodeType))
	if config.OrchestratorIP != "" {
		machine.SetOrchestratorIP(config.OrchestratorIP)
	}

	return &Provisioner{
		sshConfig: sshConfig,
		nodeType:  config.NodeType,
		config:    config,
		machine:   machine,
	}, nil
}

// Provision executes all provisioning steps
func (p *Provisioner) Provision(ctx context.Context) error {
	// Step 1: Verify SSH Connection
	if err := p.verifyConnection(ctx); err != nil {
		return fmt.Errorf("SSH connection verification failed: %w", err)
	}

	// Step 2: Install Docker and core packages
	if err := p.machine.InstallDockerAndCorePackages(ctx); err != nil {
		return fmt.Errorf("Docker installation failed: %w", err)
	}

	// Step 3: Install Bacalhau
	if err := p.installBacalhau(ctx); err != nil {
		return fmt.Errorf("Bacalhau installation failed: %w", err)
	}

	// Step 4: Configure and start Bacalhau service
	if err := p.configureService(ctx); err != nil {
		return fmt.Errorf("Bacalhau service configuration failed: %w", err)
	}

	// Step 5: Verify Installation
	if err := p.verifyInstallation(ctx); err != nil {
		return fmt.Errorf("installation verification failed: %w", err)
	}

	return nil
}

func (p *Provisioner) verifyConnection(ctx context.Context) error {
	return p.sshConfig.WaitForSSH(ctx, 10, SSHTimeOut)
}


func (p *Provisioner) installDocker(ctx context.Context) error {
	// Docker installation is handled by InstallDockerAndCorePackages
	return nil
}

func (p *Provisioner) installBacalhau(ctx context.Context) error {
	commands := []string{
		"curl -sL https://get.bacalhau.org/install.sh | bash",
	}

	for _, cmd := range commands {
		if _, err := p.sshConfig.ExecuteCommand(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provisioner) configureNode(ctx context.Context) error {
	var configCmd string
	if p.nodeType == OrchestratorNode {
		configCmd = "sudo bacalhau serve --node-type compute,requester"
	} else {
		if p.config.OrchestratorIP == "" {
			return fmt.Errorf("orchestrator IP is required for compute nodes")
		}
		configCmd = fmt.Sprintf("sudo bacalhau serve --peer %s --node-type compute", p.config.OrchestratorIP)
	}

	_, err := p.sshConfig.ExecuteCommand(ctx, configCmd)
	return err
}

func (p *Provisioner) configureService(ctx context.Context) error {
	serviceContent := `[Unit]
Description=Bacalhau Node
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/bacalhau serve
Restart=always
User=root

[Install]
WantedBy=multi-user.target`

	if err := p.sshConfig.InstallSystemdService(ctx, "bacalhau", serviceContent); err != nil {
		return err
	}

	return p.sshConfig.StartService(ctx, "bacalhau")
}

func (p *Provisioner) verifyInstallation(ctx context.Context) error {
	// Check if bacalhau is installed and running
	cmd := "systemctl is-active bacalhau"
	output, err := p.sshConfig.ExecuteCommand(ctx, cmd)
	if err != nil {
		return fmt.Errorf("bacalhau service verification failed: %w", err)
	}

	if output != "active" {
		return fmt.Errorf("bacalhau service is not active, status: %s", output)
	}

	return nil
}
