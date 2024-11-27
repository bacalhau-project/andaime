// File: update_processor.go

package display

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
)

// Constants for update types
const (
	UpdateTypeComplete UpdateType = "Complete"
	UpdateTypeResource UpdateType = "Resource"
	UpdateTypeService  UpdateType = "Service"
)

// Type definitions
type (
	UpdateType   string
	ResourceType string
	ServiceType  string
)

// UpdateAction represents an action to update the display
type UpdateAction struct {
	MachineName string
	UpdateData  UpdateData
	UpdateFunc  func(models.Machiner, UpdateData)
}

// UpdateData contains data for an update action
type UpdateData struct {
	UpdateType    UpdateType
	ResourceType  ResourceType
	ResourceState models.MachineResourceState
	ServiceType   ServiceType
	ServiceState  models.ServiceState
	Complete      bool
}

// String returns a string representation of the UpdatePayload.
func (u *UpdateData) String() string {
	if u.UpdateType == UpdateTypeResource {
		return fmt.Sprintf("%s: %s", u.UpdateType, u.ResourceType)
	}
	return fmt.Sprintf("%s: %s", u.UpdateType, u.ServiceType)
}

// NewUpdateAction creates a new UpdateAction instance.
func NewUpdateAction(machineName string, updateData UpdateData) UpdateAction {
	l := logger.Get()
	updateFunc := func(machine models.Machiner, update UpdateData) {
		switch update.UpdateType {
		case UpdateTypeResource:
			machine.SetMachineResourceState(
				string(update.ResourceType),
				models.MachineResourceState(update.ResourceState),
			)
		case UpdateTypeService:
			machine.SetServiceState(
				string(update.ServiceType),
				models.ServiceState(update.ServiceState),
			)
		default:
			l.Errorf("Invalid update type: %s", update.UpdateType)
		}
	}
	return UpdateAction{
		MachineName: machineName,
		UpdateData:  updateData,
		UpdateFunc:  updateFunc,
	}
}

// batchedUpdatesAppliedMsg is a message indicating that batched updates have been applied
type batchedUpdatesAppliedMsg struct{}

// applyBatchedUpdatesCmd returns a command to apply batched updates
func (m *DisplayModel) applyBatchedUpdatesCmd() tea.Cmd {
	l := logger.Get()
	return func() tea.Msg {
		atomic.AddInt64(&m.goroutineCount, 1)
		defer atomic.AddInt64(&m.goroutineCount, -1)

		if m.Quitting {
			l.Debug("Quitting, skipping batch updates")
			return tea.Quit
		}

		m.applyBatchedUpdates()
		return batchedUpdatesAppliedMsg{}
	}
}

// applyBatchedUpdates applies all batched updates
func (m *DisplayModel) applyBatchedUpdates() {
	if len(m.BatchedUpdates) == 0 {
		return
	}
	logger.Get().Debug(fmt.Sprintf("Applying %d batched updates", len(m.BatchedUpdates)))
	for _, update := range m.BatchedUpdates {
		m.UpdateStatus(update.Status)
	}
	m.BatchedUpdates = nil
	logger.Get().Debug("Finished applying batched updates")
}

// ProcessUpdate processes a single update action
func (m *DisplayModel) ProcessUpdate(update UpdateAction) {
	l := logger.Get()
	m.UpdateMutex.Lock()
	defer m.UpdateMutex.Unlock()

	machine, ok := m.Deployment.Machines[update.MachineName]
	if !ok {
		l.Debug(fmt.Sprintf("ProcessUpdate: Machine %s not found", update.MachineName))
		return
	}

	if update.UpdateFunc == nil {
		l.Error("ProcessUpdate: UpdateFunc is nil")
		return
	}

	switch update.UpdateData.UpdateType {
	case UpdateTypeComplete:
		machine.SetComplete()
	case UpdateTypeResource:
		machine.SetMachineResourceState(
			string(update.UpdateData.ResourceType),
			update.UpdateData.ResourceState,
		)
	case UpdateTypeService:
		machine.SetServiceState(
			string(update.UpdateData.ServiceType),
			models.ServiceState(update.UpdateData.ServiceState),
		)
	default:
		l.Errorf("ProcessUpdate: Unknown UpdateType %s", update.UpdateData.UpdateType)
		return
	}

	update.UpdateFunc(machine, update.UpdateData)
}

// QueueUpdate queues an update to be processed
func (m *DisplayModel) QueueUpdate(update UpdateAction) {
	l := logger.Get()
	select {
	case m.UpdateQueue <- update:
	default:
		l.Warn("Update queue is full, dropping update")
	}
}

// StartUpdateProcessor begins processing updates
func (m *DisplayModel) StartUpdateProcessor(ctx context.Context) {
	l := logger.Get()
	l.Debug("StartUpdateProcessor: Started")
	defer l.Debug("StartUpdateProcessor: Finished")

	for {
		select {
		case <-ctx.Done():
			l.Debug("StartUpdateProcessor: Context cancelled")
			return
		case update, ok := <-m.UpdateQueue:
			if !ok {
				l.Debug("StartUpdateProcessor: Update queue closed")
				return
			}
			l.Debugf(
				"Processing update for %s, %v",
				update.MachineName,
				update.UpdateData,
			)
			m.ProcessUpdate(update)
		}
	}
}

// StopUpdateProcessor halts the update processor
func (m *DisplayModel) StopUpdateProcessor() {
	close(m.UpdateQueue)
	<-m.UpdateProcessorDone
}
