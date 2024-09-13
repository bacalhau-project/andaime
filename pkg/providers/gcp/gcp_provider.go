package gcp

import (
	"context"
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

func (p *GCPProvider) startUpdateProcessor(ctx context.Context) {
	l := logger.Get()
	if p == nil {
		l.Debug("startUpdateProcessor: Provider is nil")
		return
	}
	p.updateProcessorDone = make(chan struct{})
	l.Debug("startUpdateProcessor: Started")
	defer close(p.updateProcessorDone)
	defer l.Debug("startUpdateProcessor: Finished")
	for {
		select {
		case <-ctx.Done():
			l.Debug("startUpdateProcessor: Context cancelled")
			return
		case update, ok := <-p.updateQueue:
			if !ok {
				l.Debug("startUpdateProcessor: Update queue closed")
				return
			}
			l.Debug(
				fmt.Sprintf(
					"startUpdateProcessor: Processing update for %s, %s",
					update.MachineName,
					update.UpdateData.ResourceType,
				),
			)
			p.processUpdate(update)
		}
	}
}

func (p *GCPProvider) processUpdate(update UpdateAction) {
	l := logger.Get()
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil || m.Deployment.Machines == nil {
		l.Debug("processUpdate: Global model, deployment, or machines is nil")
		return
	}

	machine, ok := m.Deployment.Machines[update.MachineName]
	if !ok {
		l.Debug(fmt.Sprintf("processUpdate: Machine %s not found", update.MachineName))
		return
	}

	if update.UpdateFunc == nil {
		l.Error("processUpdate: UpdateFunc is nil")
		return
	}

	if update.UpdateData.UpdateType == UpdateTypeComplete {
		machine.SetComplete()
	} else if update.UpdateData.UpdateType == UpdateTypeResource {
		machine.SetMachineResourceState(
			update.UpdateData.ResourceType.ResourceString,
			update.UpdateData.ResourceState,
		)
	} else if update.UpdateData.UpdateType == UpdateTypeService {
		machine.SetServiceState(update.UpdateData.ServiceType.Name, update.UpdateData.ServiceState)
	}

	update.UpdateFunc(machine, update.UpdateData)
}
