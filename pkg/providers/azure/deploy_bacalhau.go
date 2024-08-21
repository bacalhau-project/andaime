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
	orchestratorNodeName := ""
	numberOfOrchestratorNodes := 0

	allMachinesOrchestratorSet := false
	for name, machine := range m.Deployment.Machines {
		if machine.OrchestratorIP != "" {
			allMachinesOrchestratorSet = true
			orchestratorNodeName = name
		}
	}

	if m.Deployment.OrchestratorIP != "" || allMachinesOrchestratorSet {
		return fmt.Errorf("orchestrator IP set")
	}

	for nodeName, machine := range m.Deployment.Machines {
		if machine.Name == "" {
			return fmt.Errorf("no machine name found")
		}
		if machine.IsOrchestrator() {
			orchestratorNodeName = nodeName
			numberOfOrchestratorNodes++
			if numberOfOrchestratorNodes > 1 {
				return fmt.Errorf("multiple orchestrator nodes found")
			}
		}
	}

	if _, ok := m.Deployment.Machines[orchestratorNodeName]; !ok {
		return fmt.Errorf("no orchestrator node found")
	}

	orchestratorMachine := m.Deployment.Machines[orchestratorNodeName]
	if orchestratorMachine != nil {
		orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateNotStarted)

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

		orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateUpdating)

		getNodeConfigMetadataPath := "/tmp/get-node-config-metadata.sh"
		getNodeMetadataScriptBytes, err := internal.GetGetNodeConfigMetadataScript()
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateUpdating)
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
				"MachineType":   orchestratorMachine.VMSize,
				"MachineName":   orchestratorMachine.Name,
				"Location":      orchestratorMachine.Location,
				"Orchestrators": orchestratorMachine.PublicIP,
				"IP":            orchestratorMachine.PublicIP,
				"Token":         "",
				"NodeType":      "requester",
				"Project":       m.Deployment.ProjectID,
			},
		)
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		err = sshConfig.PushFile(
			getNodeMetadataScriptBuffer.Bytes(),
			getNodeConfigMetadataPath,
			true,
		)
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}
		_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", getNodeConfigMetadataPath))
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		installBacalhauScriptRemotePath := "/tmp/install-bacalhau.sh"
		installBacalhauScriptBytes, err := internal.GetInstallBacalhauScript()
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		err = sshConfig.PushFile(installBacalhauScriptBytes, installBacalhauScriptRemotePath, true)
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}
		_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", installBacalhauScriptRemotePath))
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		installRunBacalhauScriptRemotePath := "/tmp/install-run-bacalhau.sh"
		installRunBacalhauScriptBytes, err := internal.GetInstallRunBacalhauScript()
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		err = sshConfig.PushFile(
			installRunBacalhauScriptBytes,
			installRunBacalhauScriptRemotePath,
			true,
		)
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}
		_, err = sshConfig.ExecuteCommand(
			fmt.Sprintf("sudo %s", installRunBacalhauScriptRemotePath),
		)
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		bacalhauServiceContent, err := internal.GetBacalhauServiceScript()
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}
		sshConfig.InstallSystemdService("bacalhau", string(bacalhauServiceContent))
		sshConfig.RestartService("bacalhau")

		// Execute final test to see if the service is running
		out, err := sshConfig.ExecuteCommand("bacalhau node list --output json --api-host 0.0.0.0")
		if err != nil {
			l.Errorf("Failed to list Bacalhau nodes: %v", err)
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			return err
		}

		// Try to unmarshal the output into a JSON array
		var nodes []map[string]interface{}
		err = json.Unmarshal([]byte(out), &nodes)
		if err != nil {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			l.Errorf("Output is not valid JSON. Raw output: %s", out)
			return fmt.Errorf("failed to unmarshal node list output: %v", err)
		}

		// Check if nodes array is empty
		if len(nodes) == 0 {
			orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateFailed)
			l.Errorf("Valid JSON but no nodes found. Output: %s", out)
			return fmt.Errorf("no Bacalhau nodes found in the output")
		}

		// If the output is valid and contains nodes, log success
		l.Infof("Bacalhau orchestrator deployed successfully, nodes found: %d", len(nodes))
		orchestratorMachine.SetServiceState("Bacalhau", models.ServiceStateSucceeded)
		m.Deployment.OrchestratorIP = m.Deployment.Machines[orchestratorNodeName].PublicIP
	}

	return nil
}

func (p *AzureProvider) DeployBacalhauWorkers(
	ctx context.Context,
) error {
	l := logger.Get()
	m := display.GetGlobalModel()

	var eg errgroup.Group
	eg.SetLimit(10)

	for machineName, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}

		eg.Go(func() error {
			return func(machineName string) error {
				machine := m.Deployment.Machines[machineName]
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

				machine.SetServiceState("Bacalhau", models.ServiceStateUpdating)

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
						"MachineType":   machine.VMSize,
						"MachineName":   machine.Name,
						"Location":      machine.Location,
						"Orchestrators": m.Deployment.OrchestratorIP,
						"IP":            machine.PublicIP,
						"Token":         "",
						"NodeType":      "compute",
						"Project":       m.Deployment.ProjectID,
					},
				)
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateUpdating)
					return err
				}
				err = sshConfig.PushFile(
					getNodeMetadataScriptBuffer.Bytes(),
					getNodeConfigMetadataPath,
					true,
				)
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}
				_, err = sshConfig.ExecuteCommand(fmt.Sprintf("sudo %s", getNodeConfigMetadataPath))
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}

				installRunBacalhauScriptRemotePath := "/tmp/install-run-bacalhau.sh"
				installRunBacalhauScriptBytes, err := internal.GetInstallRunBacalhauScript()
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}

				err = sshConfig.PushFile(
					installRunBacalhauScriptBytes,
					installRunBacalhauScriptRemotePath,
					true,
				)
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}
				_, err = sshConfig.ExecuteCommand(
					fmt.Sprintf("sudo %s", installRunBacalhauScriptRemotePath),
				)
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}

				bacalhauServiceContent, err := internal.GetBacalhauServiceScript()
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}
				err = sshConfig.InstallSystemdService("bacalhau", string(bacalhauServiceContent))
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}

				err = sshConfig.RestartService("bacalhau")
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}

				// Execute final test to see if the service is running
				out, err := sshConfig.ExecuteCommand(
					"bacalhau node list --output json --api-host 0.0.0.0",
				)
				if err != nil {
					l.Errorf("Failed to list Bacalhau nodes: %v", err)
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					return err
				}

				// Try to unmarshal the output into a JSON array
				var nodes []map[string]interface{}
				err = json.Unmarshal([]byte(out), &nodes)
				if err != nil {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					l.Errorf("Output is not valid JSON. Raw output: %s", out)
					return fmt.Errorf("failed to unmarshal node list output: %v", err)
				}

				// Check if nodes array is empty
				if len(nodes) == 0 {
					machine.SetServiceState("Bacalhau", models.ServiceStateFailed)
					l.Errorf("Valid JSON but no nodes found. Output: %s", out)
					return fmt.Errorf("no Bacalhau nodes found in the output")
				}

				// If the output is valid and contains nodes, log success
				l.Infof("Bacalhau orchestrator deployed successfully, nodes found: %d", len(nodes))
				machine.SetServiceState("Bacalhau", models.ServiceStateSucceeded)
				m.Deployment.OrchestratorIP = m.Deployment.Machines[machineName].PublicIP

				return nil
			}(machineName)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}
