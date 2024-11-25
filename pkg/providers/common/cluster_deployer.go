package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	internal "github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/goroutine"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	common_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"golang.org/x/sync/errgroup"
)

// UpdateCallback is a function type for status updates during provisioning
type UpdateCallback func(*models.DisplayStatus)

// ClusterDeployer struct that implements ClusterDeployerInterface
type ClusterDeployer struct {
	sshClient sshutils.SSHClienter
	provider  models.DeploymentType
}

func NewClusterDeployer(provider models.DeploymentType) *ClusterDeployer {
	return &ClusterDeployer{
		provider: provider,
	}
}

func (cd *ClusterDeployer) SetSSHClient(client sshutils.SSHClienter) {
	cd.sshClient = client
}

func (cd *ClusterDeployer) ProvisionBacalhauCluster(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	orchestrator, err := cd.FindOrchestratorMachine()
	if err != nil {
		l.Errorf("Failed to find orchestrator machine: %v", err)
		return err
	}

	// Provision Bacalhau orchestrator
	if err := cd.ProvisionOrchestrator(ctx, orchestrator.GetName()); err != nil {
		l.Errorf("Failed to provision Bacalhau orchestrator: %v", err)
		return err
	}

	if orchestrator.GetPublicIP() == "" {
		l.Errorf("Orchestrator machine has no public IP: %v", err)
		return err
	}
	m.Deployment.OrchestratorIP = orchestrator.GetPublicIP()

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(models.NumberOfSimultaneousProvisionings)
	for _, machine := range m.Deployment.GetMachines() {
		if machine.IsOrchestrator() {
			continue
		}
		internalMachine := machine
		errGroup.Go(func() error {
			goRoutineID := goroutine.RegisterGoroutine(
				fmt.Sprintf("DeployBacalhauWorker-%s", internalMachine.GetName()),
			)
			defer goroutine.DeregisterGoroutine(goRoutineID)

			if err := cd.ProvisionWorker(ctx, internalMachine.GetName()); err != nil {
				return fmt.Errorf(
					"failed to provision Bacalhau worker %s: %v",
					internalMachine.GetName(),
					err,
				)
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("failed to provision Bacalhau: %v", err)
	}

	return nil
}

func (cd *ClusterDeployer) ProvisionOrchestrator(ctx context.Context, machineName string) error {
	if cd == nil {
		return fmt.Errorf("ClusterDeployer is nil")
	}
	m := display.GetGlobalModelFunc()
	orchestratorMachine := m.Deployment.GetMachine(machineName)
	if orchestratorMachine == nil {
		return fmt.Errorf("orchestrator machine is nil")
	}

	sshConfig, err := cd.createSSHConfig(orchestratorMachine)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	orchestratorMachine.SetOrchestratorIP("0.0.0.0")
	orchestratorMachine.SetNodeType(models.BacalhauNodeTypeOrchestrator)

	err = cd.ProvisionBacalhauNode(ctx,
		sshConfig,
		orchestratorMachine,
		m.Deployment.BacalhauSettings,
	)
	if err != nil {
		return err
	}

	orchestratorIP := orchestratorMachine.GetPublicIP()
	m.Deployment.OrchestratorIP = orchestratorIP
	err = m.Deployment.UpdateMachine(orchestratorMachine.GetName(), func(mach models.Machiner) {
		mach.SetOrchestratorIP(orchestratorIP)
	})
	if err != nil {
		return fmt.Errorf("failed to set orchestrator IP on Orchestrator node: %w", err)
	}
	for _, machine := range m.Deployment.GetMachines() {
		if machine.IsOrchestrator() {
			continue
		}
		machine.SetNodeType(models.BacalhauNodeTypeCompute)
		err = m.Deployment.UpdateMachine(machine.GetName(), func(mach models.Machiner) {
			mach.SetOrchestratorIP(orchestratorIP)
		})
		if err != nil {
			return fmt.Errorf(
				"failed to set orchestrator IP on compute node (%s): %w",
				machine.GetName(),
				err,
			)
		}
	}

	return nil
}

func (cd *ClusterDeployer) ProvisionWorker(
	ctx context.Context,
	machineName string,
) error {
	m := display.GetGlobalModelFunc()

	machine := m.Deployment.GetMachine(machineName)
	if machine.IsOrchestrator() {
		l := logger.Get()
		l.Errorf(
			"machine %s is an orchestrator, and should not be deployed as a worker",
			machineName,
		)
		return fmt.Errorf(
			"machine %s is an orchestrator, and should not be deployed as a worker",
			machineName,
		)
	}

	sshConfig, err := cd.createSSHConfig(machine)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	return cd.ProvisionBacalhauNode(
		ctx,
		sshConfig,
		machine,
		m.Deployment.BacalhauSettings,
	)
}

func (cd *ClusterDeployer) FindOrchestratorMachine() (models.Machiner, error) {
	m := display.GetGlobalModelFunc()

	var orchestratorMachine models.Machiner
	orchestratorCount := 0

	for _, machine := range m.Deployment.GetMachines() {
		if machine.IsOrchestrator() {
			orchestratorMachine = machine
			orchestratorCount++
		}
	}

	if orchestratorCount == 0 {
		return nil, fmt.Errorf("no orchestrator node found")
	}
	if orchestratorCount > 1 {
		return nil, fmt.Errorf("multiple orchestrator nodes found")
	}

	return orchestratorMachine, nil
}

// ProvisionBacalhauNode provisions a Bacalhau node on the specified machine.
// It performs a series of steps to configure, install, and verify the Bacalhau service.
//
// Parameters:
//   - ctx: The context for controlling the function's lifecycle.
//   - machine: The machine on which the Bacalhau node will be provisioned.
//   - nodeType: The type of the node, e.g., "compute".
//   - orchestratorIP: The IP address of the orchestrator, required if nodeType is "compute".
//
// Returns:
//   - error: An error if any step in the provisioning process fails.
//
// The function performs the following steps:
//  1. Creates an SSH configuration for the machine.
//  2. Sets the service state to updating.
//  3. Validates the orchestrator IP if the node type is "compute".
//  4. Sets up node configuration metadata.
//  5. Installs the Bacalhau service.
//  6. Installs the Bacalhau run script.
//  7. Sets up the Bacalhau service.
//  8. Verifies the Bacalhau deployment.
//  9. Applies Bacalhau configurations.
//  10. Executes any custom scripts.
//  11. Restarts the Bacalhau service to ensure everything is functioning correctly.
//
// If any step fails, the function handles the deployment error and returns the error.
// Upon successful completion, it logs the success, updates the service state to succeeded,
// and marks the machine as complete.
func (cd *ClusterDeployer) ProvisionBacalhauNode(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	machine models.Machiner,
	bacalhauSettings []models.BacalhauSettings,
) error {
	return cd.ProvisionBacalhauNodeWithCallback(
		ctx,
		sshConfig,
		machine,
		bacalhauSettings,
		nil,
	)
}

// sendStepUpdate is a helper function to send consistent status updates
func (cd *ClusterDeployer) sendStepUpdate(
	step common_interface.StepMessage,
	callback UpdateCallback,
	isComplete bool,
	args ...interface{},
) {
	var msg string
	if isComplete {
		if len(args) > 0 {
			msg = step.RenderDoneMessage(args...)
		} else {
			msg = step.RenderDoneMessage()
		}
	} else {
		if len(args) > 0 {
			msg = step.RenderStartMessage(args...)
		} else {
			msg = step.RenderStartMessage()
		}
	}

	callback(&models.DisplayStatus{
		StatusMessage: msg,
		StageComplete: isComplete,
	})
}

// sendErrorUpdate is a helper function to send error status updates
func (cd *ClusterDeployer) sendErrorUpdate(
	step common_interface.StepMessage,
	callback UpdateCallback,
	err error,
	args ...interface{},
) {
	callback(&models.DisplayStatus{
		StatusMessage: fmt.Sprintf("❌ %s: %v", fmt.Sprintf(step.StartMessage, args...), err),
		StageComplete: false,
	})
}

func (cd *ClusterDeployer) ProvisionBacalhauNodeWithCallback(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	machine models.Machiner,
	bacalhauSettings []models.BacalhauSettings,
	callback UpdateCallback,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	stepRegistry := common_interface.NewStepRegistry()
	machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateUpdating)

	if callback == nil {
		callback = func(status *models.DisplayStatus) {
			fmt.Printf("\r%s", status.StatusMessage)
		}
	}

	// Initial validation
	if machine.GetNodeType() != models.BacalhauNodeTypeCompute &&
		machine.GetNodeType() != models.BacalhauNodeTypeOrchestrator {
		return cd.HandleDeploymentError(
			ctx,
			machine,
			fmt.Errorf("invalid node type: %s", machine.GetNodeType()),
		)
	}

	if machine.GetNodeType() == models.BacalhauNodeTypeCompute &&
		machine.GetOrchestratorIP() == "" {
		return cd.HandleDeploymentError(ctx, machine, fmt.Errorf("no orchestrator IP found"))
	}

	// Start provisioning
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.NodeProvisioning),
		callback,
		false,
		machine.GetName(),
		machine.GetPublicIP(),
	)
	if err := cd.ProvisionMachine(ctx, sshConfig, machine); err != nil {
		l.Errorf("Machine provisioning failed for %s: %v", machine.GetName(), err)
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.NodeProvisioning),
			callback,
			err,
			machine.GetName(),
			machine.GetPublicIP(),
		)
		return err
	}
	l.Infof("Machine provisioning completed successfully for %s", machine.GetName())
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.NodeProvisioning),
		callback,
		true,
	)

	// Node configuration
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.NodeConfiguration),
		callback,
		false,
	)
	if err := cd.SetupNodeConfigMetadata(ctx, machine, sshConfig); err != nil {
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.NodeConfiguration),
			callback,
			err,
		)
		return cd.HandleDeploymentError(ctx, machine, err)
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.NodeConfiguration),
		callback,
		true,
	)

	// Bacalhau installation
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.BacalhauInstall),
		callback,
		false,
	)
	if err := cd.InstallBacalhau(ctx, sshConfig); err != nil {
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.BacalhauInstall),
			callback,
			err,
		)
		return cd.HandleDeploymentError(ctx, machine, err)
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.BacalhauInstall),
		callback,
		true,
	)

	// Run script installation
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.ServiceScript),
		callback,
		false,
	)
	if err := cd.InstallBacalhauRunScript(ctx, sshConfig); err != nil {
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.ServiceScript),
			callback,
			err,
		)
		return cd.HandleDeploymentError(ctx, machine, err)
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.ServiceScript),
		callback,
		true,
	)

	// Service setup
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.SystemdService),
		callback,
		false,
	)
	if err := cd.SetupBacalhauService(ctx, sshConfig); err != nil {
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.SystemdService),
			callback,
			err,
		)
		return cd.HandleDeploymentError(ctx, machine, err)
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.SystemdService),
		callback,
		true,
	)

	// Deployment verification
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.NodeVerification),
		callback,
		false,
	)
	if err := cd.VerifyBacalhauDeployment(ctx, sshConfig, machine.GetOrchestratorIP()); err != nil {
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.NodeVerification),
			callback,
			err,
		)
		return cd.HandleDeploymentError(ctx, machine, err)
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.NodeVerification),
		callback,
		true,
	)

	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.ConfigurationApply),
		callback,
		false,
		len(bacalhauSettings),
	)

	// Configuration application
	if len(bacalhauSettings) > 0 {
		if err := cd.ApplyBacalhauConfigs(ctx, sshConfig, bacalhauSettings); err != nil {
			cd.sendErrorUpdate(
				stepRegistry.GetStep(common_interface.ConfigurationApply),
				callback,
				err,
			)
			return cd.HandleDeploymentError(ctx, machine, err)
		}
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.ConfigurationApply),
		callback,
		true,
	)

	// Restart service after applying configurations
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.ServiceRestart),
		callback,
		false,
	)

	if err := sshConfig.RestartService(ctx, "bacalhau"); err != nil {
		cd.sendErrorUpdate(
			stepRegistry.GetStep(common_interface.ServiceRestart),
			callback,
			err,
		)
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.ServiceRestart),
		callback,
		true,
	)

	// Custom script execution
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.RunningCustomScript),
		callback,
		false,
	)
	if m.Deployment.CustomScriptPath != "" {
		if err := cd.ExecuteCustomScript(ctx, sshConfig, machine); err != nil {
			cd.sendErrorUpdate(
				stepRegistry.GetStep(common_interface.RunningCustomScript),
				callback,
				err,
			)
			return cd.HandleDeploymentError(ctx, machine, err)
		}
	}
	cd.sendStepUpdate(
		stepRegistry.GetStep(common_interface.RunningCustomScript),
		callback,
		true,
	)

	l.Infof("Bacalhau node deployed successfully on machine: %s", machine.GetName())
	machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateSucceeded)
	machine.SetComplete()

	return nil
}

func (cd *ClusterDeployer) createSSHConfig(machine models.Machiner) (sshutils.SSHConfiger, error) {
	return sshutils.NewSSHConfigFunc(
		machine.GetPublicIP(),
		machine.GetSSHPort(),
		machine.GetSSHUser(),
		machine.GetSSHPrivateKeyPath(),
	)
}

func (cd *ClusterDeployer) SetupNodeConfigMetadata(
	ctx context.Context,
	machine models.Machiner,
	sshConfig sshutils.SSHConfiger,
) error {
	m := display.GetGlobalModelFunc()

	getNodeMetadataScriptBytes, err := internal.GetGetNodeConfigMetadataScript()
	if err != nil {
		return fmt.Errorf("failed to get node config metadata script: %w", err)
	}

	// MAKE SURE WE ARE IMPORTING text/template and not html/template
	// Create a template with custom delimiters to avoid conflicts with bash syntax
	tmpl := template.New("getNodeMetadataScript")
	tmpl, err = tmpl.Delims("[[", "]]").Parse(string(getNodeMetadataScriptBytes))
	if err != nil {
		return fmt.Errorf("failed to parse node metadata script template: %w", err)
	}

	// Runtime check
	if _, ok := interface{}(tmpl).(*template.Template); !ok {
		return fmt.Errorf("incorrect template package used: expected text/template")
	}

	orchestrators := []string{}
	if machine.GetNodeType() == models.BacalhauNodeTypeOrchestrator {
		orchestrators = append(orchestrators, "0.0.0.0")
	} else if machine.GetOrchestratorIP() != "" {
		orchestrators = append(orchestrators, machine.GetOrchestratorIP())
	} else if m.Deployment.OrchestratorIP != "" {
		orchestrators = append(orchestrators, m.Deployment.OrchestratorIP)
	} else {
		return fmt.Errorf("no orchestrator IP found")
	}

	if machine.GetNodeType() != models.BacalhauNodeTypeCompute &&
		machine.GetNodeType() != models.BacalhauNodeTypeOrchestrator {
		return fmt.Errorf("invalid node type: %s", machine.GetNodeType())
	}

	var projectID string
	if m.Deployment.DeploymentType == models.DeploymentTypeAzure {
		projectID = m.Deployment.Azure.ResourceGroupName
	} else if m.Deployment.DeploymentType == models.DeploymentTypeGCP {
		projectID = m.Deployment.GetProjectID()
	}

	var scriptBuffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&scriptBuffer, "getNodeMetadataScript", map[string]interface{}{
		"MachineType":   machine.GetVMSize(),
		"MachineName":   machine.GetName(),
		"Location":      machine.GetLocation(),
		"Orchestrators": strings.Join(orchestrators, ","),
		"IP":            machine.GetPublicIP(),
		"Token":         "",
		"NodeType":      machine.GetNodeType(),
		"ProjectID":     projectID,
	})
	if err != nil {
		return fmt.Errorf("failed to execute node metadata script template: %w", err)
	}

	scriptPath := "/tmp/get-node-config-metadata.sh"
	if err := sshConfig.PushFile(ctx, scriptPath, scriptBuffer.Bytes(), true); err != nil {
		return fmt.Errorf("failed to push node config metadata script: %w", err)
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute node config metadata script: %w", err)
	}

	return nil
}

func (cd *ClusterDeployer) InstallBacalhau(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
) error {
	installScriptBytes, err := internal.GetInstallBacalhauScript()
	if err != nil {
		return fmt.Errorf("failed to get install Bacalhau script: %w", err)
	}

	scriptPath := "/tmp/install-bacalhau.sh"
	if err := sshConfig.PushFile(ctx, scriptPath, installScriptBytes, true); err != nil {
		return fmt.Errorf("failed to push install Bacalhau script: %w", err)
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute install Bacalhau script: %w", err)
	}

	return nil
}

func (cd *ClusterDeployer) InstallBacalhauRunScript(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
) error {
	installScriptBytes, err := internal.GetInstallRunBacalhauScript()
	if err != nil {
		return fmt.Errorf("failed to get install Bacalhau script: %w", err)
	}

	scriptPath := "/tmp/install-run-bacalhau.sh"
	if err := sshConfig.PushFile(ctx, scriptPath, installScriptBytes, true); err != nil {
		return fmt.Errorf("failed to push install Bacalhau script: %w", err)
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute install Bacalhau script: %w", err)
	}

	return nil
}

func (cd *ClusterDeployer) SetupBacalhauService(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
) error {
	serviceContent, err := internal.GetBacalhauServiceScript()
	if err != nil {
		return fmt.Errorf("failed to get Bacalhau service script: %w", err)
	}

	if err := sshConfig.InstallSystemdService(ctx, "bacalhau", string(serviceContent)); err != nil {
		return fmt.Errorf("failed to install Bacalhau systemd service: %w", err)
	}

	// Always restart after service installation
	if err := sshConfig.RestartService(ctx, "bacalhau"); err != nil {
		return fmt.Errorf("failed to restart Bacalhau service: %w", err)
	}

	return nil
}

func (cd *ClusterDeployer) VerifyBacalhauDeployment(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	orchestratorIP string,
) error {
	l := logger.Get()
	if orchestratorIP == "" {
		orchestratorIP = "0.0.0.0"
	}
	out, err := sshConfig.ExecuteCommand(
		ctx,
		fmt.Sprintf("bacalhau node list --output json --api-host %s", orchestratorIP),
	)
	if err != nil {
		return fmt.Errorf("failed to list Bacalhau nodes: %w", err)
	}

	nodes, err := utils.StripAndParseJSON(out)
	if err != nil {
		return fmt.Errorf("failed to strip and parse JSON: %w", err)
	}

	if len(nodes) == 0 {
		l.Errorf("Valid JSON but no nodes found. Output: %s", out)
		return fmt.Errorf("no Bacalhau nodes found in the output")
	}

	l.Infof(
		"Bacalhau node verified on machine %s, nodes found: %d",
		orchestratorIP,
		len(nodes),
	)
	return nil
}

func (cd *ClusterDeployer) ExecuteCustomScript(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	machine models.Machiner,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	if m.Deployment.CustomScriptPath == "" {
		l.Info("No custom script path provided, skipping execution")
		return nil
	}

	scriptContent, err := os.ReadFile(m.Deployment.CustomScriptPath)
	if err != nil {
		return fmt.Errorf("failed to read custom script: %w", err)
	}

	remotePath := "/tmp/custom_script.sh"
	if err := sshConfig.PushFile(ctx, remotePath, scriptContent, true); err != nil {
		return fmt.Errorf("failed to push custom script: %w", err)
	}

	logFile := "/var/log/andaime-custom-script.log"
	cmd := fmt.Sprintf("sudo bash %s | sudo tee %s", remotePath, logFile)
	if _, err := sshConfig.ExecuteCommand(ctx, cmd); err != nil {
		return fmt.Errorf("failed to execute custom script: %w", err)
	}

	l.Infof("Custom script executed successfully on machine: %s", machine.GetName())
	machine.SetServiceState(models.ServiceTypeScript.Name, models.ServiceStateSucceeded)
	return nil
}

func (cd *ClusterDeployer) ApplyBacalhauConfigs(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	combinedSettings []models.BacalhauSettings,
) error {
	l := logger.Get()
	l.Info("Applying Bacalhau configurations")

	if err := applySettings(ctx, sshConfig, combinedSettings); err != nil {
		return fmt.Errorf("failed to apply Bacalhau configs: %w", err)
	}

	if err := verifySettings(ctx, sshConfig, combinedSettings); err != nil {
		return fmt.Errorf("failed to verify Bacalhau configs: %w", err)
	}

	l.Info("Bacalhau configurations applied and verified")
	return nil
}

func combineSettings(settings []models.BacalhauSettings) map[string]string {
	combined := make(map[string]string)
	for _, setting := range settings {
		switch v := setting.Value.(type) {
		case []string:
			combined[setting.Key] = strings.Join(v, ",")
		case string:
			combined[setting.Key] = v
		}
	}
	return combined
}

func applySettings(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	settings []models.BacalhauSettings,
) error {
	l := logger.Get()
	for _, setting := range settings {
		var valueStr string
		switch v := setting.Value.(type) {
		case string:
			valueStr = v
		case []string:
			valueStr = strings.Join(v, ",")
		case []interface{}:
			var values []string
			for _, item := range v {
				values = append(values, fmt.Sprintf("%v", item))
			}
			valueStr = strings.Join(values, ",")
		default:
			valueStr = fmt.Sprintf("%v", v)
		}

		cmd := fmt.Sprintf("sudo bacalhau config set '%s'='%s'", setting.Key, valueStr)
		output, err := sshConfig.ExecuteCommand(ctx, cmd)
		if err != nil {
			if strings.Contains(
				output,
				fmt.Sprintf("invalid configuration key \"%s\": not found", setting.Key),
			) {
				l.Errorf("Bad setting detected: %s", setting.Key)
				return fmt.Errorf("bad setting detected: %s", setting.Key)
			}
			l.Errorf("Failed to apply Bacalhau config %s: %v", setting.Key, err)
			return fmt.Errorf("failed to apply Bacalhau config %s: %w", setting.Key, err)
		}
		l.Infof("Applied Bacalhau config %s: %s", setting.Key, output)
	}
	return nil
}

func verifySettings(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	expectedSettings []models.BacalhauSettings,
) error {
	l := logger.Get()
	output, err := sshConfig.ExecuteCommand(ctx, "sudo bacalhau config list --output json")
	if err != nil {
		return fmt.Errorf("failed to get Bacalhau config: %w", err)
	}

	var configList []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &configList); err != nil {
		return fmt.Errorf("failed to parse Bacalhau config: %w", err)
	}

	for _, expectedSetting := range expectedSettings {
		found := false
		for _, config := range configList {
			if config["Key"] == expectedSetting.Key {
				actualValue := fmt.Sprintf("%v", config["Value"])
				if strings.HasPrefix(actualValue, "[") && strings.HasSuffix(actualValue, "]") {
					actualValue = strings.TrimPrefix(actualValue, "[")
					actualValue = strings.TrimSuffix(actualValue, "]")
				}
				if values, ok := config["Value"].([]string); ok {
					for _, value := range values {
						if value == expectedSetting.Value {
							found = true
							break
						}
					}
				} else if actualValue != expectedSetting.Value {
					l.Warnf(
						"Bacalhau config %s has unexpected value. Expected: %s, Actual: %s",
						expectedSetting.Key,
						expectedSetting.Value,
						actualValue,
					)
				} else {
					l.Debugf("Verified Bacalhau config %s: %s", expectedSetting.Key, actualValue)
				}
				break
			}
		}
		if !found {
			l.Warnf("Bacalhau config %s not found in the final configuration", expectedSetting.Key)
		}
	}

	return nil
}

func (cd *ClusterDeployer) HandleDeploymentError(
	_ context.Context,
	machine models.Machiner,
	err error,
) error {
	l := logger.Get()
	machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateFailed)
	l.Errorf("Failed to deploy Bacalhau on machine %s: %v", machine.GetName(), err)
	return err
}

func (cd *ClusterDeployer) ProvisionMachine(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
	machine models.Machiner,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil {
		return fmt.Errorf("global display model is not initialized")
	}

	status := models.NewDisplayStatusWithText(
		machine.GetName(),
		models.AzureResourceTypeVM,
		models.ResourceStatePending,
		"Provisioning Docker & packages on machine",
	)
	m.UpdateStatus(status)
	err := machine.InstallDockerAndCorePackages(ctx)
	if err != nil {
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machine.GetName(),
			models.AzureResourceTypeVM,
			models.ResourceStateFailed,
			fmt.Sprintf("Failed to provision Docker & packages on machine: %v", err),
		))
		return err
	}
	m.UpdateStatus(models.NewDisplayStatusWithText(
		machine.GetName(),
		models.AzureResourceTypeVM,
		models.ResourceStateSucceeded,
		"Provisioned Docker & packages on machine",
	))
	return nil
}

func (cd *ClusterDeployer) WaitForAllMachinesToReachState(
	ctx context.Context,
	resourceType string,
	state models.MachineResourceState,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for {
		allReady := true
		for _, machine := range m.Deployment.GetMachines() {
			if machine.GetMachineResourceState(resourceType) != state {
				allReady = false
				break
			}
		}
		if allReady {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			l.Debugf("Waiting for all machines to reach state: %d", state)
		}
	}
}

var _ common_interface.ClusterDeployerer = &ClusterDeployer{}
