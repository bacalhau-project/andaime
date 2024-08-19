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

	m := display.GetGlobalModel()
	var orchestratorNodeIndex int = -1

	for i := range m.Deployment.Machines {
		if m.Deployment.Machines[i].IsOrchestrator() {
			if orchestratorNodeIndex == -1 {
				return fmt.Errorf("multiple orchestrator nodes found")
			}
			orchestratorNodeIndex = i
		}
	}

	return nil
}

func (p *AzureProvider) DeployBacalhauWorkers(ctx context.Context) error {
	return nil
}
