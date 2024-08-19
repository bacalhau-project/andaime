package azure

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func (p *AzureProvider) DeployBacalhauOrchestrator(ctx context.Context) error {
	l := logger.Get()
	l.Info("Deploying Bacalhau Orchestrator")

	m := display.GetGlobalModelFunc()
	var orchestratorNodeIndex int = -1
	var numberOfOrchestratorNodes int = 0

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

	m.Deployment.OrchestratorIP = m.Deployment.Machines[orchestratorNodeIndex].PublicIP

	return nil
}

func (p *AzureProvider) DeployBacalhauWorkers(ctx context.Context) error {
	return nil
}
