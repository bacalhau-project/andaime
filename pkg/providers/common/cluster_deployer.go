package common

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	internal "github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

type ClusterDeployerInterface interface {
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)
	GetSSHClient() sshutils.SSHClienter
	SetSSHClient(client sshutils.SSHClienter)

	CreateResources(ctx context.Context) error

	ProvisionSSH(ctx context.Context) error
	WaitForAllMachinesToReachState(
		ctx context.Context,
		resourceType string,
		state models.ResourceState,
	) error
	ProvisionPackagesOnMachine(ctx context.Context, machineName string) error

	ProvisionBacalhau(ctx context.Context) error

	ProvisionOrchestrator(ctx context.Context, machineName string) error
	ProvisionWorker(ctx context.Context, machineName string) error
}

// ClusterDeployer struct that implements ClusterDeployerInterface
type ClusterDeployer struct {
	config    *viper.Viper
	sshClient sshutils.SSHClienter
}

func NewClusterDeployer() *ClusterDeployer {
	return &ClusterDeployer{}
}

// Implement the interface methods
func (cd *ClusterDeployer) GetConfig() *viper.Viper {
	return cd.config
}

func (cd *ClusterDeployer) SetConfig(config *viper.Viper) {
	cd.config = config
}

func (cd *ClusterDeployer) GetSSHClient() sshutils.SSHClienter {
	return cd.sshClient
}

func (cd *ClusterDeployer) SetSSHClient(client sshutils.SSHClienter) {
	cd.sshClient = client
}

func (cd *ClusterDeployer) ProvisionBacalhau(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	// Provision Bacalhau orchestrator
	if err := cd.DeployOrchestrator(ctx); err != nil {
		l.Errorf("Failed to provision Bacalhau orchestrator: %v", err)
		return err
	}

	orchestrator, err := cd.FindOrchestratorMachine()
	if err != nil {
		l.Errorf("Failed to find orchestrator machine: %v", err)
		return err
	}

	if orchestrator.PublicIP == "" {
		l.Errorf("Orchestrator machine has no public IP: %v", err)
		return err
	}
	m.Deployment.OrchestratorIP = orchestrator.PublicIP

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(models.NumberOfSimultaneousProvisionings)
	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}
		internalMachine := machine
		errGroup.Go(func() error {
			goRoutineID := m.RegisterGoroutine(
				fmt.Sprintf("DeployBacalhauWorker-%s", internalMachine.Name),
			)
			defer m.DeregisterGoroutine(goRoutineID)

			if err := cd.DeployWorker(ctx, internalMachine.Name); err != nil {
				return fmt.Errorf(
					"failed to provision Bacalhau worker %s: %v",
					internalMachine.Name,
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

func (cd *ClusterDeployer) DeployOrchestrator(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	orchestratorMachine, err := cd.FindOrchestratorMachine()
	if err != nil {
		return err
	}

	err = cd.DeployBacalhauNode(ctx, orchestratorMachine.Name, "requester")
	if err != nil {
		return err
	}

	orchestratorIP := orchestratorMachine.PublicIP
	m.Deployment.OrchestratorIP = orchestratorIP
	err = m.Deployment.UpdateMachine(orchestratorMachine.Name, func(mach *models.Machine) {
		mach.OrchestratorIP = orchestratorIP
	})
	if err != nil {
		return fmt.Errorf("failed to set orchestrator IP on Orchestrator node: %w", err)
	}
	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}
		err = m.Deployment.UpdateMachine(machine.Name, func(mach *models.Machine) {
			mach.OrchestratorIP = orchestratorIP
		})
		if err != nil {
			return fmt.Errorf(
				"failed to set orchestrator IP on compute node (%s): %w",
				machine.Name,
				err,
			)
		}
	}

	return nil
}

func (cd *ClusterDeployer) DeployWorker(
	ctx context.Context,
	machineName string,
) error {
	m := display.GetGlobalModelFunc()

	if m.Deployment.Machines[machineName].Orchestrator {
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

	return cd.DeployBacalhauNode(ctx, machineName, "compute")
}

func (cd *ClusterDeployer) FindOrchestratorMachine() (*models.Machine, error) {
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

func (cd *ClusterDeployer) DeployBacalhauNode(
	ctx context.Context,
	machineName string,
	nodeType string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	machine := m.Deployment.Machines[machineName]
	sshConfig, err := cd.createSSHConfig(machine)
	if err != nil {
		return err
	}

	machine.SetServiceState("Bacalhau", models.ServiceStateUpdating)

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

	if err := cd.VerifyBacalhauDeployment(ctx, sshConfig, machine.OrchestratorIP); err != nil {
		return cd.HandleDeploymentError(ctx, machine, err)
	}

	l.Infof("Bacalhau node deployed successfully on machine: %s", machine.Name)
	machine.SetServiceState("Bacalhau", models.ServiceStateSucceeded)

	if machine.Orchestrator {
		m.Deployment.OrchestratorIP = machine.PublicIP
		machine.OrchestratorIP = "0.0.0.0"
	}

	machine.SetComplete()

	return nil
}

func (cd *ClusterDeployer) createSSHConfig(machine *models.Machine) (sshutils.SSHConfiger, error) {
	return sshutils.NewSSHConfigFunc(
		machine.PublicIP,
		machine.SSHPort,
		machine.SSHUser,
		machine.SSHPrivateKeyPath,
	)
}

func (cd *ClusterDeployer) SetupNodeConfigMetadata(
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
	} else if machine.OrchestratorIP != "" {
		orchestrators = append(orchestrators, machine.OrchestratorIP)
	} else if m.Deployment.OrchestratorIP != "" {
		orchestrators = append(orchestrators, m.Deployment.OrchestratorIP)
	} else {
		return fmt.Errorf("no orchestrator IP found")
	}

	var scriptBuffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&scriptBuffer, "getNodeMetadataScript", map[string]interface{}{
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

func (cd *ClusterDeployer) HandleDeploymentError(
	_ context.Context,
	machine *models.Machine,
	err error,
) error {
	l := logger.Get()
	machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
	l.Errorf("Failed to deploy Bacalhau on machine %s: %v", machine.Name, err)
	return err
}

func (cd *ClusterDeployer) ProvisionPackagesOnMachine(
	ctx context.Context,
	machineName string,
) error {
	m := display.GetGlobalModelFunc()
	mach := m.Deployment.Machines[machineName]

	m.UpdateStatus(models.NewDisplayStatusWithText(
		machineName,
		models.AzureResourceTypeVM,
		models.ResourceStatePending,
		"Provisioning Docker & packages on machine",
	))
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
	state models.ResourceState,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for {
		allReady := true
		for _, machine := range m.Deployment.Machines {
			if machine.GetResourceState(resourceType) != state {
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
