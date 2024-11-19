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

// Provision executes all provisioning steps with progress updates
func (p *Provisioner) Provision(ctx context.Context) error {
	// Create a channel for progress updates
	updates := make(chan *models.DisplayStatus)
	defer close(updates)

	// Start a goroutine to handle progress updates
	go func() {
		for status := range updates {
			if status.DetailedStatus != "" {
				fmt.Printf(
					"\r%s (%s) [%d%%]",
					status.StatusMessage,
					status.DetailedStatus,
					status.Progress,
				)
			} else {
				fmt.Printf("\r%s [%d%%]", status.StatusMessage, status.Progress)
			}
			fmt.Println()
		}
	}()

	return p.ProvisionWithCallback(ctx, func(status *models.DisplayStatus) {
		updates <- status
	})
}

// ProvisionWithCallback executes all provisioning steps with callback updates
func (p *Provisioner) ProvisionWithCallback(
	ctx context.Context,
	callback common.UpdateCallback,
) error {
	progress := models.NewProvisionProgress()
	l := logger.Get()
	callback(&models.DisplayStatus{
		StatusMessage: "🚀 Starting node provisioning process",
		Progress:      0,
	})

	if ctx == nil {
		l.Error("Context is nil")
		callback(&models.DisplayStatus{
			StatusMessage: "❌ Provisioning failed: context is nil",
			Progress:      0,
		})
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
		StatusMessage: "📡 Establishing SSH connection...",
		Progress:      int(progress.GetProgress()),
	})

	if err := p.SSHConfig.WaitForSSH(ctx, 3, SSHTimeOut); err != nil {
		progress.CurrentStep.Status = "Failed"
		progress.CurrentStep.Error = err
		errMsg := fmt.Sprintf("❌ SSH connection failed: %v", err)
		fmt.Println(errMsg)
		callback(&models.DisplayStatus{
			StatusMessage:  errMsg,
			DetailedStatus: err.Error(),
			Progress:       int(progress.GetProgress()),
		})
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	progress.CurrentStep.Status = "Completed"
	progress.AddStep(progress.CurrentStep)
	callback(&models.DisplayStatus{
		StatusMessage: "✅ SSH connection established successfully",
		Progress:      int(progress.GetProgress()),
	})

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
	callback(&models.DisplayStatus{
		StatusMessage: fmt.Sprintf("🔧 Provisioning node on %s", p.Config.IPAddress),
		Progress:      int(progress.GetProgress()),
	})
	callback(&models.DisplayStatus{
		StatusMessage: "Installing Docker and dependencies...",
		Progress:      int(progress.GetProgress()),
	})
	if err := cd.ProvisionBacalhauNodeWithCallback(
		ctx,
		p.SSHConfig,
		p.Machine,
		settings,
		callback,
	); err != nil {
		callback(&models.DisplayStatus{
			StatusMessage: fmt.Sprintf("❌ Failed to provision node (ip: %s, user: %s)",
				p.Config.IPAddress,
				p.Config.Username),
			Progress: int(progress.GetProgress()),
		})

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
	callback(&models.DisplayStatus{
		StatusMessage: fmt.Sprintf("✅ Successfully provisioned node on %s", p.Config.IPAddress),
		Progress:      100,
	})

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
	if err := provisioner.ProvisionWithCallback(cmd.Context(), func(status *models.DisplayStatus) {
		if status.DetailedStatus != "" {
			fmt.Printf("\r%s (%s) [%d%%]\n", status.StatusMessage, status.DetailedStatus, status.Progress)
		} else {
			fmt.Printf("\r%s [%d%%]\n", status.StatusMessage, status.Progress)
		}
	}); err != nil {
		l.Errorf("Provisioning failed: %v", err)
		return fmt.Errorf("provisioning failed: %w", err)
	}
	l.Info("Provisioning completed successfully")

	return nil
}
