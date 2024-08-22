package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	internal "github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"golang.org/x/sync/errgroup"
)

type BacalhauDeployer struct{}

func NewBacalhauDeployer() *BacalhauDeployer {
	return &BacalhauDeployer{}
}

func (p *AzureProvider) DeployBacalhauOrchestrator(ctx context.Context) error {
	deployer := NewBacalhauDeployer()
	return deployer.DeployOrchestrator(ctx)
}

func (p *AzureProvider) DeployBacalhauWorkers(ctx context.Context) error {
	deployer := NewBacalhauDeployer()
	return deployer.DeployWorkers(ctx)
}

func (bd *BacalhauDeployer) DeployOrchestrator(ctx context.Context) error {
	orchestratorMachine, err := bd.findOrchestratorMachine()
	if err != nil {
		return err
	}

	return bd.deployBacalhauNode(ctx, orchestratorMachine, "requester")
}

func (bd *BacalhauDeployer) DeployWorkers(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	var eg errgroup.Group
	eg.SetLimit(10)

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}

		machine := machine // Create a new variable to avoid closure issues
		eg.Go(func() error {
			return bd.deployBacalhauNode(ctx, machine, "compute")
		})
	}

	return eg.Wait()
}

func (bd *BacalhauDeployer) findOrchestratorMachine() (*models.Machine, error) {
	m := display.GetGlobalModelFunc()

	var orchestratorMachine *models.Machine
	orchestratorCount := 0

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
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

func (bd *BacalhauDeployer) deployBacalhauNode(
	ctx context.Context,
	machine *models.Machine,
	nodeType string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	sshConfig, err := bd.createSSHConfig(machine)
	if err != nil {
		return err
	}

	machine.SetServiceState("Bacalhau", models.ServiceStateUpdating)

	if err := bd.setupNodeConfigMetadata(ctx, machine, sshConfig, nodeType); err != nil {
		return bd.handleDeploymentError(machine, err)
	}

	if err := bd.installBacalhau(ctx, sshConfig); err != nil {
		return bd.handleDeploymentError(machine, err)
	}

	if err := bd.installBacalhauRunScript(ctx, sshConfig); err != nil {
		return bd.handleDeploymentError(machine, err)
	}

	if err := bd.setupBacalhauService(ctx, sshConfig); err != nil {
		return bd.handleDeploymentError(machine, err)
	}

	if machine.OrchestratorIP == "" {
		machine.OrchestratorIP = m.Deployment.OrchestratorIP
	}
	if err := bd.verifyBacalhauDeployment(ctx, sshConfig, machine.OrchestratorIP); err != nil {
		return bd.handleDeploymentError(machine, err)
	}

	l.Infof("Bacalhau node deployed successfully on machine: %s", machine.Name)
	machine.SetServiceState("Bacalhau", models.ServiceStateSucceeded)

	if machine.Orchestrator {
		m.Deployment.OrchestratorIP = machine.PublicIP
	}

	return nil
}

func (bd *BacalhauDeployer) createSSHConfig(machine *models.Machine) (sshutils.SSHConfiger, error) {
	return sshutils.NewSSHConfigFunc(
		machine.PublicIP,
		machine.SSHPort,
		machine.SSHUser,
		machine.SSHPrivateKeyMaterial,
	)
}

func (bd *BacalhauDeployer) setupNodeConfigMetadata(
	ctx context.Context,
	machine *models.Machine,
	sshConfig sshutils.SSHConfiger,
	nodeType string,
) error {
	m := display.GetGlobalModelFunc()

	getNodeMetadataScriptBytes, err := internal.GetGetNodeConfigMetadataScript()
	if err != nil {
		return fmt.Errorf("failed to get node config metadata script: %w", err)
	}

	tmpl, err := template.New("getNodeMetadataScript").Parse(string(getNodeMetadataScriptBytes))
	if err != nil {
		return fmt.Errorf("failed to parse node metadata script template: %w", err)
	}

	orchestrators := []string{m.Deployment.OrchestratorIP}
	if machine.Orchestrator {
		orchestrators = append(orchestrators, strings.TrimSpace(machine.PublicIP))
	}

	var scriptBuffer bytes.Buffer
	err = tmpl.Execute(&scriptBuffer, map[string]interface{}{
		"MachineType":   machine.VMSize,
		"MachineName":   machine.Name,
		"Location":      machine.Location,
		"Orchestrators": strings.Join(orchestrators, ","),
		"IP":            machine.PublicIP,
		"Token":         "",
		"NodeType":      nodeType,
		"Project":       m.Deployment.ProjectID,
	})
	if err != nil {
		return fmt.Errorf("failed to execute node metadata script template: %w", err)
	}

	scriptPath := "/tmp/get-node-config-metadata.sh"
	if err := sshConfig.PushFile(ctx, scriptBuffer.Bytes(), scriptPath, true); err != nil {
		return fmt.Errorf("failed to push node config metadata script: %w", err)
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute node config metadata script: %w", err)
	}

	return nil
}

func (bd *BacalhauDeployer) installBacalhau(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
) error {
	installScriptBytes, err := internal.GetInstallBacalhauScript()
	if err != nil {
		return fmt.Errorf("failed to get install Bacalhau script: %w", err)
	}

	scriptPath := "/tmp/install-bacalhau.sh"
	if err := sshConfig.PushFile(ctx, installScriptBytes, scriptPath, true); err != nil {
		return fmt.Errorf("failed to push install Bacalhau script: %w", err)
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute install Bacalhau script: %w", err)
	}

	return nil
}

func (bd *BacalhauDeployer) installBacalhauRunScript(
	ctx context.Context,
	sshConfig sshutils.SSHConfiger,
) error {
	installScriptBytes, err := internal.GetInstallRunBacalhauScript()
	if err != nil {
		return fmt.Errorf("failed to get install Bacalhau script: %w", err)
	}

	scriptPath := "/tmp/install-run-bacalhau.sh"
	if err := sshConfig.PushFile(ctx, installScriptBytes, scriptPath, true); err != nil {
		return fmt.Errorf("failed to push install Bacalhau script: %w", err)
	}

	if _, err := sshConfig.ExecuteCommand(ctx, fmt.Sprintf("sudo %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute install Bacalhau script: %w", err)
	}

	return nil
}

func (bd *BacalhauDeployer) setupBacalhauService(
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

func (bd *BacalhauDeployer) verifyBacalhauDeployment(
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

	var nodes []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &nodes); err != nil {
		l.Errorf("Output is not valid JSON. Raw output: %s", out)
		return fmt.Errorf("failed to unmarshal node list output: %w", err)
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

func (bd *BacalhauDeployer) handleDeploymentError(machine *models.Machine, err error) error {
	l := logger.Get()
	machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
	l.Errorf("Failed to deploy Bacalhau on machine %s: %v", machine.Name, err)
	return err
}
