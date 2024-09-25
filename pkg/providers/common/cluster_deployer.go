package common

import (
	"bytes"
	"context"
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
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

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

func (cd *ClusterDeployer) ProvisionAllMachinesWithPackages(ctx context.Context) error {
	m := display.GetGlobalModelFunc()

	var errGroup errgroup.Group
	for _, machine := range m.Deployment.Machines {
		internalMachine := machine
		errGroup.Go(func() error {
			goRoutineID := goroutine.RegisterGoroutine(
				fmt.Sprintf("ProvisionPackagesOnMachine-%s", internalMachine.GetName()),
			)
			defer goroutine.DeregisterGoroutine(goRoutineID)
			m.UpdateStatus(models.NewDisplayStatusWithText(
				internalMachine.GetName(),
				models.AzureResourceTypeVM,
				models.ResourceStatePending,
				"Provisioning Docker & packages on machine",
			))
			err := cd.ProvisionPackagesOnMachine(ctx, internalMachine.GetName())
			if err != nil {
				m.UpdateStatus(models.NewDisplayStatusWithText(
					internalMachine.GetName(),
					models.AzureResourceTypeVM,
					models.ResourceStateFailed,
					fmt.Sprintf("Failed to provision Docker & packages on machine: %v", err),
				))
				return fmt.Errorf(
					"failed to provision packages on machine %s: %v",
					internalMachine.GetName(),
					err,
				)
			}
			m.UpdateStatus(models.NewDisplayStatusWithText(
				internalMachine.GetName(),
				models.AzureResourceTypeVM,
				models.ResourceStateSucceeded,
				"Provisioned Docker & packages on machine",
			))
			return nil
		})
	}
	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("failed to provision packages on all machines: %v", err)
	}
	return nil
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

	err := cd.provisionBacalhauNode(ctx, orchestratorMachine.GetName(), "requester")
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

	if m.Deployment.GetMachine(machineName).IsOrchestrator() {
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

	return cd.provisionBacalhauNode(ctx, machineName, "compute")
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

func (cd *ClusterDeployer) provisionBacalhauNode(
	ctx context.Context,
	machineName string,
	nodeType string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	machine := m.Deployment.GetMachine(machineName)
	sshConfig, err := cd.createSSHConfig(machine)
	if err != nil {
		return err
	}

	machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateUpdating)

	if nodeType == "compute" && m.Deployment.OrchestratorIP == "" {
		return cd.HandleDeploymentError(ctx, machine, fmt.Errorf("no orchestrator IP found"))
	}

	if err := cd.SetupNodeConfigMetadata(ctx, machine, sshConfig, nodeType); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	if err := cd.InstallBacalhau(ctx, sshConfig); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	if err := cd.InstallBacalhauRunScript(ctx, sshConfig); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	if err := cd.SetupBacalhauService(ctx, sshConfig); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	if err := cd.VerifyBacalhauDeployment(ctx, sshConfig, machine.GetOrchestratorIP()); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	if err := cd.ExecuteCustomScript(ctx, sshConfig, machine); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	l.Infof("Bacalhau node deployed successfully on machine: %s", machine.GetName())
	machine.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateSucceeded)

	if machine.IsOrchestrator() {
		m.Deployment.OrchestratorIP = machine.GetPublicIP()
		machine.SetOrchestratorIP("0.0.0.0")
	}

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
	nodeType string,
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
	if nodeType == "requester" {
		orchestrators = append(orchestrators, "0.0.0.0")
	} else if machine.GetOrchestratorIP() != "" {
		orchestrators = append(orchestrators, machine.GetOrchestratorIP())
	} else if m.Deployment.OrchestratorIP != "" {
		orchestrators = append(orchestrators, m.Deployment.OrchestratorIP)
	} else {
		return fmt.Errorf("no orchestrator IP found")
	}

	var scriptBuffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&scriptBuffer, "getNodeMetadataScript", map[string]interface{}{
		"MachineType":   machine.GetVMSize(),
		"MachineName":   machine.GetName(),
		"Location":      machine.GetLocation(),
		"Orchestrators": strings.Join(orchestrators, ","),
		"IP":            machine.GetPublicIP(),
		"Token":         "",
		"NodeType":      nodeType,
		"Project":       m.Deployment.ProjectID,
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
	customScriptPath := viper.GetString("general.custom_script_path")
	if customScriptPath == "" {
		l.Info("No custom script path provided, skipping execution")
		return nil
	}

	scriptContent, err := os.ReadFile(customScriptPath)
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

func (cd *ClusterDeployer) ProvisionPackagesOnMachine(
	ctx context.Context,
	machineName string,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil {
		return fmt.Errorf("global display model is not initialized")
	}
	mach := m.Deployment.GetMachine(machineName)
	if mach == nil {
		return fmt.Errorf("machine %s not found", machineName)
	}

	status := models.NewDisplayStatusWithText(
		machineName,
		models.AzureResourceTypeVM,
		models.ResourceStatePending,
		"Provisioning Docker & packages on machine",
	)
	m.UpdateStatus(status)
	err := mach.InstallDockerAndCorePackages(ctx)
	if err != nil {
		m.UpdateStatus(models.NewDisplayStatusWithText(
			machineName,
			models.AzureResourceTypeVM,
			models.ResourceStateFailed,
			fmt.Sprintf("Failed to provision Docker & packages on machine: %v", err),
		))
		return err
	}
	m.UpdateStatus(models.NewDisplayStatusWithText(
		machineName,
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
		for _, machine := range m.Deployment.Machines {
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
