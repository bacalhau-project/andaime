package provision

import (
	"context"
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

const (
	SSHTimeOut     = 60 * time.Second
	DefaultSSHPort = 22
)

// Provisioner handles the node provisioning process
type Provisioner struct {
	sshConfig      sshutils.SSHConfiger
	config         *NodeConfig
	machine        models.Machiner
	settingsParser *SettingsParser
	deployer       common_interface.ClusterDeployerer
}

// NewProvisioner creates a new Provisioner instance
func NewProvisioner(config *NodeConfig) (*Provisioner, error) {
	if config == nil {
		return nil, fmt.Errorf("node config cannot be nil")
	}

	if err := validateNodeConfig(config); err != nil {
		return nil, fmt.Errorf("invalid node configuration: %w", err)
	}

	sshConfig, err := sshutils.NewSSHConfigFunc(
		config.IPAddress,
		DefaultSSHPort,
		config.Username,
		config.PrivateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH config: %w", err)
	}

	machine, err := createMachineInstance(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine instance: %w", err)
	}

	return &Provisioner{
		sshConfig:      sshConfig,
		config:         config,
		machine:        machine,
		settingsParser: NewSettingsParser(),
	}, nil
}

// validateNodeConfig checks if the provided configuration is valid
func validateNodeConfig(config *NodeConfig) error {
	if config.IPAddress == "" {
		return fmt.Errorf("IP address is required")
	}
	if config.Username == "" {
		return fmt.Errorf("username is required")
	}
	if config.PrivateKey == "" {
		return fmt.Errorf("private key is required")
	}
	return nil
}

// createMachineInstance creates a new machine instance with the provided configuration
func createMachineInstance(config *NodeConfig) (models.Machiner, error) {
	machine := models.Machine{}
	machine.SetSSHUser(config.Username)
	machine.SetSSHPrivateKeyPath(config.PrivateKey)
	machine.SetSSHPort(DefaultSSHPort)
	machine.SetPublicIP(config.IPAddress)
	machine.SetOrchestratorIP("")
	machine.SetOrchestrator(true)
	machine.SetNodeType(models.BacalhauNodeTypeOrchestrator)

	if config.OrchestratorIP != "" {
		machine.SetOrchestratorIP(config.OrchestratorIP)
		machine.SetOrchestrator(false)
		machine.SetNodeType(models.BacalhauNodeTypeCompute)
	}

	return &machine, nil
}

// Provision executes all provisioning steps
func (p *Provisioner) Provision(ctx context.Context) error {
	l := logger.Get()
	l.Info("Starting node provisioning process")

	if ctx == nil {
		l.Error("Context is nil")
		return fmt.Errorf("context cannot be nil")
	}

	ctx, cancel := context.WithTimeout(ctx, SSHTimeOut)
	defer cancel()

	// Ensure SSH connection is available before proceeding
	l.Infof("Establishing SSH connection to %s...", p.config.IPAddress)
	if err := p.sshConfig.WaitForSSH(ctx, 3, SSHTimeOut); err != nil {
		l.Errorf("Failed to establish SSH connection: %v", err)
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	l.Info("SSH connection established successfully")

	cd := common.NewClusterDeployer(models.DeploymentTypeUnknown)
	l.Debug("Created cluster deployer")

	// Parse settings if path is provided
	l.Debug("Parsing Bacalhau settings")
	settings, err := p.settingsParser.ParseFile(p.config.BacalhauSettingsPath)
	if err != nil {
		l.Errorf("Failed to parse settings: %v", err)
		return err
	}
	if len(settings) > 0 {
		l.Infof("Found %d Bacalhau settings to apply", len(settings))
	}

	// Provision the node
	l.Infof("Starting Bacalhau node provisioning on %s...", p.config.IPAddress)
	if err := cd.ProvisionBacalhauNode(
		ctx,
		p.sshConfig,
		p.machine,
		settings,
	); err != nil {
		l.Errorf("Failed to provision Bacalhau node (ip: %s, user: %s)", 
			p.config.IPAddress,
			p.config.Username)
		
		// Extract command output if it's an SSH error
		var cmdOutput string
		if sshErr, ok := err.(*sshutils.SSHError); ok {
			cmdOutput = sshErr.Output
		}
		
		// Log detailed error information
		l.Errorf("Provisioning failed with error: %v", err)
		if cmdOutput != "" {
			l.Errorf("Command output: %s", cmdOutput)
		}
		
		l.Debugf("Full error context:\nIP: %s\nUser: %s\nPrivate Key Path: %s\nError: %v", 
			p.config.IPAddress,
			p.config.Username,
			p.config.PrivateKey,
			err)
		if ctx.Err() != nil {
			l.Debugf("Context error: %v", ctx.Err())
		}
		
		if cmdOutput != "" {
			return fmt.Errorf("failed to provision Bacalhau node:\nIP: %s\nCommand Output: %s\nError Details: %w",
				p.config.IPAddress, cmdOutput, err)
		}
		return fmt.Errorf("failed to provision Bacalhau node:\nIP: %s\nError Details: %w",
			p.config.IPAddress, err)
	}
	l.Infof("Successfully provisioned Bacalhau node on %s", p.config.IPAddress)

	return nil
}

// GetMachine returns the configured machine instance
func (p *Provisioner) GetMachine() models.Machiner {
	return p.machine
}

// GetSSHConfig returns the configured SSH configuration
func (p *Provisioner) GetSSHConfig() sshutils.SSHConfiger {
	return p.sshConfig
}

// GetSettings returns the configured Bacalhau settings
func (p *Provisioner) GetSettings() ([]models.BacalhauSettings, error) {
	return p.settingsParser.ParseFile(p.config.BacalhauSettingsPath)
}

// GetConfig returns the configured node configuration
func (p *Provisioner) GetConfig() *NodeConfig {
	return p.config
}

// SetClusterDeployer sets the cluster deployer for the provisioner
func (p *Provisioner) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.deployer = deployer
}

// ParseSettings parses the Bacalhau settings from the given file path
func (p *Provisioner) ParseSettings(filePath string) ([]models.BacalhauSettings, error) {
	return p.settingsParser.ParseFile(filePath)
}
