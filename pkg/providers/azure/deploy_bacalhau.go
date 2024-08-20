package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	internal "github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"golang.org/x/sync/errgroup"
)

func (p *AzureProvider) DeployBacalhauOrchestrator(ctx context.Context) error {
	l := logger.Get()
	l.Info("Deploying Bacalhau Orchestrator")

	m := display.GetGlobalModelFunc()
	orchestratorNodeIndex := -1
	numberOfOrchestratorNodes := 0

	allMachinesOrchestratorSet := false
	for i := range m.Deployment.Machines {
		if m.Deployment.Machines[i].OrchestratorIP != "" {
			allMachinesOrchestratorSet = true
		}
	}

	if m.Deployment.OrchestratorIP != "" || allMachinesOrchestratorSet {
		return fmt.Errorf("orchestrator IP set")
	}

	for i := range m.Deployment.Machines {
		if m.Deployment.Machines[i].IsOrchestrator() {
			numberOfOrchestratorNodes++
			if numberOfOrchestratorNodes > 1 {
				return fmt.Errorf("multiple orchestrator nodes found")
			}
			orchestratorNodeIndex = i
		}
	}

	if orchestratorNodeIndex == -1 && m.Deployment.OrchestratorIP == "" {
		return fmt.Errorf("no orchestrator node found")
	}

	orchestratorMachine := m.Deployment.Machines[orchestratorNodeIndex]

	bacalhauService := orchestratorMachine.MachineServices["Bacalhau"]
	bacalhauService.State = models.ServiceStateNotStarted
	m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService

	sshConfig, err := sshutils.NewSSHConfigFunc(
		orchestratorMachine.PublicIP,
		orchestratorMachine.SSHPort,
		orchestratorMachine.SSHUser,
		orchestratorMachine.SSHPrivateKeyMaterial,
	)
	if err != nil {
		l.Errorf("Error creating SSH config: %v", err)
		return err
	}

	bacalhauService.State = models.ServiceStateUpdating
	m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService

	getNodeConfigMetadataPath := "/tmp/get-node-config-metadata.sh"
	getNodeMetadataScriptBytes, err := internal.GetGetNodeConfigMetadataScript()
	if err != nil {
		bacalhauService.State = models.ServiceStateUpdating
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}
	err = sshConfig.PushFile(getNodeMetadataScriptBytes, getNodeConfigMetadataPath, true)
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}
	_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", getNodeConfigMetadataPath))
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}

	installBacalhauScriptRemotePath := "/tmp/install-bacalhau.sh"
	installBacalhauScriptBytes, err := internal.GetInstallBacalhauScript()
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}

	err = sshConfig.PushFile(installBacalhauScriptBytes, installBacalhauScriptRemotePath, true)
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}
	_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", installBacalhauScriptRemotePath))
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}

	installBacalhauServiceScriptRemotePath := "/tmp/install-and-restart-bacalhau-service.sh"
	installBacalhauServiceScriptBytes, err := internal.GetInstallAndRestartBacalhauServicesScript()
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}
	err = sshConfig.PushFile(
		installBacalhauServiceScriptBytes,
		installBacalhauServiceScriptRemotePath,
		true,
	)
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}
	_, err = sshConfig.ExecuteCommand(
		fmt.Sprintf("sudo %s", installBacalhauServiceScriptRemotePath),
	)
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}

	// Execute final test to see if the service is running
	out, err := sshConfig.ExecuteCommand("bacalhau node list --output json --api-host 0.0.0.0")
	if err != nil {
		l.Errorf("Failed to list Bacalhau nodes: %v", err)
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		return err
	}

	// Try to unmarshal the output into a JSON array
	var nodes []map[string]interface{}
	err = json.Unmarshal([]byte(out), &nodes)
	if err != nil {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		l.Errorf("Output is not valid JSON. Raw output: %s", out)
		return fmt.Errorf("failed to unmarshal node list output: %v", err)
	}

	// Check if nodes array is empty
	if len(nodes) == 0 {
		bacalhauService.State = models.ServiceStateFailed
		m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
		l.Errorf("Valid JSON but no nodes found. Output: %s", out)
		return fmt.Errorf("no Bacalhau nodes found in the output")
	}

	// If the output is valid and contains nodes, log success
	l.Infof("Bacalhau orchestrator deployed successfully, nodes found: %d", len(nodes))
	bacalhauService.State = models.ServiceStateSucceeded
	m.Deployment.Machines[orchestratorNodeIndex].MachineServices["Bacalhau"] = bacalhauService
	m.Deployment.OrchestratorIP = m.Deployment.Machines[orchestratorNodeIndex].PublicIP

	return nil
}

func (p *AzureProvider) DeployBacalhauWorkers(
	ctx context.Context,
) error {
	l := logger.Get()
	m := display.GetGlobalModel()

	var eg errgroup.Group

	for machineIndex, machine := range m.Deployment.Machines {
		internalMachine := machine
		if machine.Orchestrator {
			continue
		}

		eg.Go(func() error {
			return func(machineIndex int, machine *models.Machine) error {
				sshConfig, err := sshutils.NewSSHConfigFunc(
					machine.PublicIP,
					machine.SSHPort,
					machine.SSHUser,
					machine.SSHPrivateKeyMaterial,
				)
				if err != nil {
					l.Errorf("Error creating SSH config: %v", err)
					return err
				}

				bacalhauService := m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"]

				bacalhauService.State = models.ServiceStateUpdating
				m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService

				getNodeConfigMetadataPath := "/tmp/get-node-config-metadata.sh"
				getNodeMetadataScriptBytes, err := internal.GetGetNodeConfigMetadataScript()
				if err != nil {
					return err
				}

				// Fill out template variables with the correct values
				getNodeMetadataScriptTemplate, err := template.New("getNodeMetadataScript").
					Parse(string(getNodeMetadataScriptBytes))
				if err != nil {
					return err
				}
				var getNodeMetadataScriptBuffer bytes.Buffer
				err = getNodeMetadataScriptTemplate.Execute(
					&getNodeMetadataScriptBuffer,
					map[string]interface{}{
						"OrchestratorIPs": m.Deployment.OrchestratorIP,
						"ProjectName":     m.Deployment.ProjectID,
					},
				)
				if err != nil {
					bacalhauService.State = models.ServiceStateUpdating
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}
				err = sshConfig.PushFile(
					getNodeMetadataScriptBytes,
					getNodeConfigMetadataPath,
					true,
				)
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}
				_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", getNodeConfigMetadataPath))
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}

				installBacalhauComputeScriptRemotePath := "/tmp/install-bacalhau-compute.sh"
				installBacalhauComputeScriptBytes, err := internal.GetInstallBacalhauComputeScript()
				if err != nil {
					return err
				}

				// Update NODE_TYPE in template to be Machine.Type
				installBacalhauComputeScriptTemplate, err := template.New("installBacalhauComputeScript").
					Parse(string(installBacalhauComputeScriptBytes))
				if err != nil {
					return err
				}

				var installBacalhauComputeScriptBuffer bytes.Buffer
				err = installBacalhauComputeScriptTemplate.Execute(
					&installBacalhauComputeScriptBuffer,
					map[string]interface{}{
						"NodeType": machine.VMSize,
					},
				)

				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}

				err = sshConfig.PushFile(
					installBacalhauComputeScriptBuffer.Bytes(),
					installBacalhauComputeScriptRemotePath,
					true,
				)
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}
				_, err = sshConfig.ExecuteCommand(
					fmt.Sprintf("sudo %s", installBacalhauComputeScriptRemotePath),
				)
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}

				installBacalhauServiceScriptRemotePath := "/tmp/install-and-restart-bacalhau-service.sh"
				installBacalhauServiceScriptBytes, err := internal.GetInstallAndRestartBacalhauServicesScript()
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}
				err = sshConfig.PushFile(
					installBacalhauServiceScriptBytes,
					installBacalhauServiceScriptRemotePath,
					true,
				)
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}
				_, err = sshConfig.ExecuteCommand(
					fmt.Sprintf("sudo %s", installBacalhauServiceScriptRemotePath),
				)
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}

				// Execute final test to see if the service is running
				out, err := sshConfig.ExecuteCommand(
					"bacalhau node list --output json --api-host 0.0.0.0",
				)
				if err != nil {
					l.Errorf("Failed to list Bacalhau nodes: %v", err)
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					return err
				}

				// Try to unmarshal the output into a JSON array
				var nodes []map[string]interface{}
				err = json.Unmarshal([]byte(out), &nodes)
				if err != nil {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					l.Errorf("Output is not valid JSON. Raw output: %s", out)
					return fmt.Errorf("failed to unmarshal node list output: %v", err)
				}

				// Check if nodes array is empty
				if len(nodes) == 0 {
					bacalhauService.State = models.ServiceStateFailed
					m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
					l.Errorf("Valid JSON but no nodes found. Output: %s", out)
					return fmt.Errorf("no Bacalhau nodes found in the output")
				}

				// If the output is valid and contains nodes, log success
				l.Infof("Bacalhau orchestrator deployed successfully, nodes found: %d", len(nodes))
				bacalhauService.State = models.ServiceStateSucceeded
				m.Deployment.Machines[machineIndex].MachineServices["Bacalhau"] = bacalhauService
				m.Deployment.OrchestratorIP = m.Deployment.Machines[machineIndex].PublicIP

				return nil
			}(machineIndex, &internalMachine)
		})
	}

	return eg.Wait()
}
