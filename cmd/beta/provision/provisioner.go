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
	"github.com/spf13/cobra"
)

const (
	SSHTimeOut     = 180 * time.Second // Increased from 60s to 180s
	DefaultSSHPort = 22
	MaxRetries     = 5
	RetryDelay     = 10 * time.Second
)

// Provisioner handles the node provisioning process
type Provisioner struct {
	SSHConfig      sshutils.SSHConfiger
	Config         *NodeConfig
	Machine        models.Machiner
	SettingsParser *SettingsParser
	Deployer       common_interface.ClusterDeployerer
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
		SSHConfig:      sshConfig,
		Config:         config,
		Machine:        machine,
		SettingsParser: NewSettingsParser(),
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
	return p.ProvisionWithCallback(ctx, func(*models.DisplayStatus) {})
}

// ProvisionWithCallback executes all provisioning steps with status updates
func (p *Provisioner) ProvisionWithCallback(
	ctx context.Context,
	callback common.UpdateCallback,
) error {
	progress := models.NewProvisionProgress()
	l := logger.Get()
	l.Info("Starting node provisioning process")

	if ctx == nil {
		l.Error("Context is nil")
		return fmt.Errorf("context cannot be nil")
	}

	ctx, cancel := context.WithTimeout(ctx, SSHTimeOut)
	defer cancel()

	// Initial Connection Step
	progress.SetCurrentStep(&models.ProvisionStep{
		Name:        "Initial Connection",
		Description: "Establishing SSH connection",
	})
	callback(&models.DisplayStatus{
		StatusMessage: "Establishing SSH connection...",
		Progress:     int(progress.GetProgress()),
	})
	
	if err := p.SSHConfig.WaitForSSH(ctx, 3, SSHTimeOut); err != nil {
		progress.CurrentStep.Status = "Failed"
		progress.CurrentStep.Error = err
		callback(&models.DisplayStatus{
			StatusMessage:  fmt.Sprintf("SSH connection failed: %v", err),
			DetailedStatus: err.Error(),
			Progress:      int(progress.GetProgress()),
		})
		l.Errorf("Failed to establish SSH connection: %v", err)
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	// Check system requirements
	l.Info("Checking system requirements...")
	callback(&models.DisplayStatus{
		StatusMessage: "Checking system requirements...",
		Progress:     int(progress.GetProgress()),
	})

	if err := checkSystemRequirements(ctx, p.SSHConfig); err != nil {
		progress.CurrentStep.Status = "Failed"
		progress.CurrentStep.Error = err
		callback(&models.DisplayStatus{
			StatusMessage:  fmt.Sprintf("System requirements check failed: %v", err),
			DetailedStatus: err.Error(),
			Progress:      int(progress.GetProgress()),
		})
		l.Errorf("System requirements check failed: %v", err)
		return fmt.Errorf("system requirements check failed: %w", err)
	}

	progress.CurrentStep.Status = "Completed"
	progress.AddStep(progress.CurrentStep)
	callback(&models.DisplayStatus{
		StatusMessage: "SSH connection established successfully",
		Progress:     int(progress.GetProgress()),
	})
	l.Info("SSH connection established successfully")

	// System Preparation Step
	progress.SetCurrentStep(&models.ProvisionStep{
		Name:        "System Preparation",
		Description: "Initializing deployment configuration",
	})
	callback(&models.DisplayStatus{
		StatusMessage: "Preparing system configuration...",
		Progress:     int(progress.GetProgress()),
	})

	// Update package lists
	session, err := p.SSHConfig.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	l.Info("Updating package lists...")
	if err := session.Run("sudo apt-get update"); err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}
	session.Close()

	// Install core dependencies
	session, err = p.SSHConfig.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	l.Info("Installing core dependencies...")
	if err := session.Run("sudo apt-get install -y curl wget apt-transport-https ca-certificates software-properties-common"); err != nil {
		return fmt.Errorf("failed to install core dependencies: %w", err)
	}
	session.Close()

	cd := common.NewClusterDeployer(models.DeploymentTypeUnknown)
	l.Debug("Created cluster deployer")
	
	progress.CurrentStep.Status = "Completed"
	progress.AddStep(progress.CurrentStep)

	// Parse settings if path is provided
	l.Debug("Parsing Bacalhau settings")
	settings, err := p.SettingsParser.ParseFile(p.Config.BacalhauSettingsPath)
	if err != nil {
		l.Errorf("Failed to parse settings: %v", err)
		return err
	}
	if len(settings) > 0 {
		l.Infof("Found %d Bacalhau settings to apply", len(settings))
	}

	// Provision the node
	l.Infof("Starting Bacalhau node provisioning on %s...", p.Config.IPAddress)
	if err := cd.ProvisionBacalhauNodeWithCallback(
		ctx,
		p.SSHConfig,
		p.Machine,
		settings,
		callback,
	); err != nil {
		l.Errorf("Failed to provision Bacalhau node (ip: %s, user: %s)",
			p.Config.IPAddress,
			p.Config.Username)

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
			p.Config.IPAddress,
			p.Config.Username,
			p.Config.PrivateKey,
			err)
		if ctx.Err() != nil {
			l.Debugf("Context error: %v", ctx.Err())
		}

		if cmdOutput != "" {
			return fmt.Errorf(
				"failed to provision Bacalhau node:\nIP: %s\nCommand Output: %s\nError Details: %w",
				p.Config.IPAddress,
				cmdOutput,
				err,
			)
		}
		return fmt.Errorf("failed to provision Bacalhau node:\nIP: %s\nError Details: %w",
			p.Config.IPAddress, err)
	}
	l.Infof("Successfully provisioned Bacalhau node on %s", p.Config.IPAddress)

	return nil
}

// GetMachine returns the configured machine instance
func (p *Provisioner) GetMachine() models.Machiner {
	return p.Machine
}

// GetSSHConfig returns the configured SSH configuration
func (p *Provisioner) GetSSHConfig() sshutils.SSHConfiger {
	return p.SSHConfig
}

// GetSettings returns the configured Bacalhau settings
func (p *Provisioner) GetSettings() ([]models.BacalhauSettings, error) {
	return p.SettingsParser.ParseFile(p.Config.BacalhauSettingsPath)
}

// GetConfig returns the configured node configuration
func (p *Provisioner) GetConfig() *NodeConfig {
	return p.Config
}

// SetClusterDeployer sets the cluster deployer for the provisioner
func (p *Provisioner) SetClusterDeployer(deployer common_interface.ClusterDeployerer) {
	p.Deployer = deployer
}

// ParseSettings parses the Bacalhau settings from the given file path
func (p *Provisioner) ParseSettings(filePath string) ([]models.BacalhauSettings, error) {
	return p.SettingsParser.ParseFile(filePath)
}

func runProvision(cmd *cobra.Command, args []string) error {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create new provisioner
	l := logger.Get()
	l.Debug("Creating new provisioner with config")
	l.Debugf("Config details - IP: %s, Username: %s", config.IPAddress, config.Username)

	provisioner, err := NewProvisioner(config)
	if err != nil {
		l.Errorf("Failed to create provisioner: %v", err)
		return fmt.Errorf("failed to create provisioner: %w", err)
	}
	l.Debug("Successfully created provisioner")

	// Run provisioning
	l.Debug("Starting provisioning process")
	if err := provisioner.Provision(cmd.Context()); err != nil {
		l.Errorf("Provisioning failed: %v", err)
		return fmt.Errorf("provisioning failed: %w", err)
	}
	l.Info("Provisioning completed successfully")

	return nil
}
