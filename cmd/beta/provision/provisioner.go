package provision

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const (
	SSHTimeOut     = 180 * time.Second // Increased from 60s to 180s
	DefaultSSHPort = 22
	MaxRetries     = 5
	RetryDelay     = 10 * time.Second
	marginSpaces   = "   "
	SSHRetryCount  = 10 // Number of SSH connection retries
)

// Provisioner handles the node provisioning process
type Provisioner struct {
	SSHConfig      sshutils_interfaces.SSHConfiger
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
		config.PrivateKeyPath,
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
	if config.PrivateKeyPath == "" {
		return fmt.Errorf("private key is required")
	}
	return nil
}

// createMachineInstance creates a new machine instance with the provided configuration
func createMachineInstance(config *NodeConfig) (models.Machiner, error) {
	machine := models.Machine{}
	machine.SetSSHUser(config.Username)
	machine.SetSSHPrivateKeyPath(config.PrivateKeyPath)
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
	return p.ProvisionWithCallback(ctx, func(status *models.DisplayStatus) {})
}

// ProvisionWithCallback executes all provisioning steps with callback updates
func (p *Provisioner) ProvisionWithCallback(
	ctx context.Context,
	callback common.UpdateCallback,
) error {
	progress := models.NewProvisionProgress()
	l := logger.Get()
	stepRegistry := common_interface.NewStepRegistry()

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
		StatusMessage: stepRegistry.GetStep(common_interface.SSHConnection).RenderStartMessage(),
	})

	if err := p.SSHConfig.WaitForSSH(ctx, SSHRetryCount, SSHTimeOut); err != nil { //nolint:mnd
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

	// Execute 'hostname' on the server to get the hostname and assign it to the machine
	hostname, err := p.SSHConfig.ExecuteCommand(ctx, "hostname")
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	hostname = strings.TrimSpace(hostname)
	p.Machine.SetName(hostname)

	progress.CurrentStep.Status = "Completed"
	progress.AddStep(progress.CurrentStep)
	callback(&models.DisplayStatus{
		StatusMessage: fmt.Sprintf(
			stepRegistry.GetStep(common_interface.SSHConnection).DoneMessage,
			p.Config.IPAddress,
		),
		StageComplete: true,
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
		})

		return handleProvisionError(err, p.Config, l)
	}

	return nil
}

// handleProvisionError processes and formats provisioning errors
func handleProvisionError(err error, config *NodeConfig, l *logger.Logger) error {
	var cmdOutput string

	type outputError interface {
		Output() string
	}

	if outputErr, ok := err.(outputError); ok {
		cmdOutput = outputErr.Output()
	}

	l.Errorf("Provisioning failed with error: %v", err)
	if cmdOutput != "" {
		l.Errorf("Command output: %s", cmdOutput)
	}

	l.Debugf("Full error context:\nIP: %s\nUser: %s\nPrivate Key Path: %s\nError: %v",
		config.IPAddress,
		config.Username,
		config.PrivateKeyPath,
		err)

	if cmdOutput != "" {
		return fmt.Errorf(
			"failed to provision Bacalhau node:\nIP: %s\nCommand Output: %s\nError Details: %w",
			config.IPAddress,
			cmdOutput,
			err)
	}

	return err
}

// GetMachine returns the configured machine instance
func (p *Provisioner) GetMachine() models.Machiner {
	return p.Machine
}

// GetSSHConfig returns the configured SSH configuration
func (p *Provisioner) GetSSHConfig() sshutils_interfaces.SSHConfiger {
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

// testMode is used to run the provisioner in test mode
var testMode bool

func runProvision(cmd *cobra.Command, args []string) error {
	// Get configuration from flags
	config := &NodeConfig{
		IPAddress:            cmd.Flag("ip").Value.String(),
		Username:             cmd.Flag("user").Value.String(),
		PrivateKeyPath:       cmd.Flag("key").Value.String(),
		OrchestratorIP:       cmd.Flag("orchestrator").Value.String(),
		BacalhauSettingsPath: cmd.Flag("bacalhau-settings").Value.String(),
	}

	cmd.Flags().BoolVar(&testMode, "test", false,
		"Run in test mode (simulation only)")

	// Validate configuration
	if allErrs := config.Validate(); len(allErrs) > 0 {
		fmt.Println("Invalid configuration:")
		for _, err := range allErrs {
			fmt.Println(err)
		}
		return fmt.Errorf("invalid configuration")
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

	// Starting message
	fmt.Printf(`Starting provisioning process for:
  IP: %s
  Username: %s
  Private Key Path: %s

`,
		config.IPAddress,
		config.Username,
		config.PrivateKeyPath,
	)

	// Create a channel for progress updates
	updates := make(chan *models.DisplayStatus)
	stepRegistry := common_interface.NewStepRegistry()

	// Create error group for handling updates
	errGroup := &errgroup.Group{}
	errGroup.Go(func() error {
		totalSteps := len(stepRegistry.GetAllSteps())
		currentStep := 0

		for status := range updates {
			// Skip empty messages
			statusMsg := strings.TrimSpace(status.StatusMessage)
			if statusMsg == "" {
				continue
			}

			// Format the progress string
			var progressStr string
			if !status.StageComplete {
				progressStr = fmt.Sprintf("[ %d of %d ]", currentStep+1, totalSteps)
			}

			// Build the status line with proper alignment
			var statusLine string
			if status.StageComplete {
				currentStep++
				statusLine = fmt.Sprintf("%s   %s", marginSpaces, statusMsg)
			} else {
				statusLine = fmt.Sprintf("%s%s", marginSpaces, statusMsg)
			}

			// Ensure consistent width and add progress
			if !status.StageComplete {
				statusLine = fmt.Sprintf("%-65s %s", statusLine, progressStr)
			}

			// Clear the line and print the new status
			fmt.Printf("\r%s\n", statusLine)
			if status.StageComplete {
				fmt.Println()
			}
		}
		return nil
	})

	// Run provisioning
	l.Debug("Starting provisioning process")

	if testMode {
		l.Info("Running in test mode - simulating deployment")
		simulateDeployment(provisioner.Config, updates)
	} else {
		// Run the actual provisioning
		provisionErr := provisioner.ProvisionWithCallback(
			cmd.Context(),
			func(ds *models.DisplayStatus) {
				updates <- ds
			},
		)
		if provisionErr != nil {
			l.Errorf("Provisioning failed: %v", provisionErr)
			return fmt.Errorf("provisioning failed: %w", provisionErr)
		}
	}

	// Close the updates channel after provisioning is done
	close(updates)

	// Wait for error group to finish processing all updates
	updateErr := errGroup.Wait()
	if updateErr != nil {
		l.Errorf("Error in progress updates: %v", updateErr)
		return fmt.Errorf("error in progress updates: %w", updateErr)
	}

	// Print success message
	l.Info("Provisioning completed successfully")
	fmt.Printf("\nSuccessfully provisioned node on:\n")
	fmt.Printf("    IP: %s\n", config.IPAddress)
	fmt.Printf("    Username: %s\n", config.Username)

	return nil
}

func simulateDeployment(config *NodeConfig, updates chan<- *models.DisplayStatus) {
	stepRegistry := common_interface.NewStepRegistry()
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()
		sendUpdate := func(msg string, stageComplete bool) {
			updates <- &models.DisplayStatus{
				StatusMessage: msg,
				StageComplete: stageComplete,
			}
			time.Sleep(20 * time.Millisecond) //nolint:mnd
		}

		randNumOfSettings := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

		// Simulate each step
		sendUpdate(stepRegistry.GetStep(common_interface.SSHConnection).RenderStartMessage(), false)
		sendUpdate(
			stepRegistry.GetStep(common_interface.SSHConnection).
				RenderDoneMessage(config.IPAddress),
			true,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.NodeProvisioning).
				RenderStartMessage(config.IPAddress),
			false,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.NodeProvisioning).RenderDoneMessage(),
			true,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.NodeConfiguration).RenderStartMessage(),
			false,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.NodeConfiguration).RenderDoneMessage(),
			true,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.BacalhauInstall).RenderStartMessage(),
			false,
		)
		sendUpdate(stepRegistry.GetStep(common_interface.BacalhauInstall).RenderDoneMessage(), true)
		sendUpdate(stepRegistry.GetStep(common_interface.ServiceScript).RenderStartMessage(), false)
		sendUpdate(stepRegistry.GetStep(common_interface.ServiceScript).RenderDoneMessage(), true)
		sendUpdate(
			stepRegistry.GetStep(common_interface.SystemdService).RenderStartMessage(),
			false,
		)
		sendUpdate(stepRegistry.GetStep(common_interface.SystemdService).RenderDoneMessage(), true)
		sendUpdate(
			stepRegistry.GetStep(common_interface.NodeVerification).RenderStartMessage(),
			false,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.NodeVerification).RenderDoneMessage(),
			true,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.ConfigurationApply).
				RenderStartMessage(randNumOfSettings.Intn(20)), //nolint:mnd
			false,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.ConfigurationApply).RenderDoneMessage(),
			true,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.ServiceRestart).RenderStartMessage(),
			false,
		)
		sendUpdate(stepRegistry.GetStep(common_interface.ServiceRestart).RenderDoneMessage(), true)
		sendUpdate(
			stepRegistry.GetStep(common_interface.RunningCustomScript).RenderStartMessage(),
			false,
		)
		sendUpdate(
			stepRegistry.GetStep(common_interface.RunningCustomScript).RenderDoneMessage(),
			true,
		)
	}()

	waitGroup.Wait()
}
