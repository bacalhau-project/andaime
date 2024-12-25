// File: display_model.go

package display

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
)

// Constants
const (
	UpdateQueueSize       = 100
	LogLines              = 10
	updateTimesBufferSize = 10
)

// tickMsg represents a tick message
type tickMsg time.Time

// logLinesMsg represents a message containing log lines
type logLinesMsg []string

// DisplayModel represents the main display model
type DisplayModel struct {
	Deployment          *models.Deployment
	TextBox             []string
	Quitting            bool
	LastUpdate          time.Time
	DebugMode           bool
	UpdateTimes         []time.Duration
	UpdateTimesIndex    int
	UpdateTimesSize     int
	LastUpdateStart     time.Time
	CPUUsage            float64
	MemoryUsage         uint64
	BatchedUpdates      []models.StatusUpdateMsg
	BatchUpdateTimer    *time.Timer
	updateBuffer        *utils.CircularBuffer[models.DisplayStatus]
	quitChan            chan bool
	goroutineCount      int64
	keyEventChan        chan tea.KeyMsg
	activeGoroutines    sync.Map
	UpdateMutex         sync.Mutex
	UpdateQueue         chan UpdateAction
	UpdateProcessorDone chan bool
}

var (
	globalModelInstance *DisplayModel
	globalModelOnce     sync.Once
)

var GetGlobalModelFunc func() *DisplayModel = GetGlobalModel

// GetGlobalModel returns the singleton instance of DisplayModel
func GetGlobalModel() *DisplayModel {
	if globalModelInstance == nil {
		globalModelOnce.Do(func() {
			deployment, err := models.NewDeployment()
			if err != nil {
				panic(err)
			}
			globalModelInstance = NewDisplayModel(deployment)
		})
	}
	return globalModelInstance
}

// SetGlobalModel sets the global DisplayModel instance
func SetGlobalModel(m *DisplayModel) {
	var setGlobalModelMutex sync.Mutex

	setGlobalModelMutex.Lock()
	defer setGlobalModelMutex.Unlock()

	globalModelInstance = m
}

// NewDisplayModel creates and returns a new DisplayModel
func NewDisplayModel(deployment *models.Deployment) *DisplayModel {
	model := &DisplayModel{
		Deployment:       deployment,
		TextBox:          []string{"Resource Status Monitor"},
		LastUpdate:       time.Now(),
		DebugMode:        os.Getenv("DEBUG_DISPLAY") == "1",
		UpdateTimes:      make([]time.Duration, updateTimesBufferSize),
		UpdateTimesIndex: 0,
		UpdateTimesSize:  updateTimesBufferSize,
		quitChan:         make(chan bool),
		keyEventChan:     make(chan tea.KeyMsg),
		updateBuffer:     utils.NewCircularBuffer[models.DisplayStatus](1000),
	}
	go model.handleKeyEvents()
	SetGlobalModel(model)
	return model
}

// handleKeyEvents processes key events in a separate goroutine
func (m *DisplayModel) handleKeyEvents() {
	l := logger.Get()
	for {
		select {
		case <-m.quitChan:
			l.Debug("Quit signal received in handleKeyEvents")
			return
		case key := <-m.keyEventChan:
			if (key.String() == "q" || key.String() == "ctrl+c") && !m.Quitting {
				m.Quitting = true
				l.Debugf(
					"Quit command received (q or ctrl+c) at %s",
					time.Now().Format(time.RFC3339Nano),
				)
				close(m.quitChan)
				l.Debug("Quit channel closed")
				return
			}
		}
	}
}

// RegisterGoroutine registers a new goroutine with a label
func (m *DisplayModel) RegisterGoroutine(label string) int64 {
	l := logger.Get()
	id := atomic.AddInt64(&m.goroutineCount, 1)
	m.activeGoroutines.Store(id, label)
	l.Debugf("Goroutine started: %s (ID: %d)", label, id)
	return id
}

// DeregisterGoroutine deregisters a goroutine by its ID
func (m *DisplayModel) DeregisterGoroutine(id int64) {
	l := logger.Get()
	if label, ok := m.activeGoroutines.LoadAndDelete(id); ok {
		l.Debugf("Goroutine finished: %s (ID: %d)", label, id)
	}
	atomic.AddInt64(&m.goroutineCount, -1)
}

// UpdateStatus updates the status of a machine
func (m *DisplayModel) UpdateStatus(status *models.DisplayStatus) {
	if m == nil || m.updateBuffer == nil || status == nil || status.Name == "" {
		return
	}

	m.updateBuffer.Add(*status)

	if status.Name != "" {
		m.updateMachineStatus(status.Name, status)

		// Explicitly update progress
		if machine, ok := m.Deployment.Machines[status.Name]; ok {
			_, _ = machine.ResourcesComplete()
			// logger.WriteToDebugLog(
			// 	fmt.Sprintf(
			// 		"UpdateStatus: Machine: %s, Progress: %d, Total: %d",
			// 		status.Name,
			// 		progress,
			// 		total,
			// 	),
			// )
		}
	}
}

func (m *DisplayModel) updateMachineStatus(machineName string, newStatus *models.DisplayStatus) {
	if !m.machineExists(machineName) {
		return
	}

	m.updateStatusMessage(machineName, newStatus)
	m.updateLocation(machineName, newStatus)
	m.updateIPs(machineName, newStatus)
	m.updateElapsedTime(machineName, newStatus)
	m.updateOrchestratorStatus(machineName, newStatus)
	m.updateServiceStates(machineName, newStatus)
}

func (m *DisplayModel) machineExists(machineName string) bool {
	l := logger.Get()
	if _, ok := m.Deployment.Machines[machineName]; !ok {
		l.Debugf("Machine %s not found, skipping update", machineName)
		return false
	}
	return true
}

func (m *DisplayModel) updateStatusMessage(machineName string, newStatus *models.DisplayStatus) {
	l := logger.Get()
	if newStatus.StatusMessage != "" {
		trimmedStatus := strings.TrimSpace(newStatus.StatusMessage)
		if len(trimmedStatus) > StatusLength-3 {
			l.Debugf("Status too long, truncating: '%s'", trimmedStatus)
			err := m.Deployment.UpdateMachine(machineName, func(m models.Machiner) {
				m.SetStatusMessage(trimmedStatus[:StatusLength-3] + "…")
			})
			if err != nil {
				l.Errorf("Error updating machine status: %v", err)
			}
		} else {
			err := m.Deployment.UpdateMachine(machineName, func(m models.Machiner) {
				m.SetStatusMessage(fmt.Sprintf("%-*s", StatusLength, trimmedStatus))
			})
			if err != nil {
				l.Errorf("Error updating machine status: %v", err)
			}
		}
	}
}

func (m *DisplayModel) updateLocation(machineName string, newStatus *models.DisplayStatus) {
	l := logger.Get()
	if newStatus.Location != "" {
		err := m.Deployment.UpdateMachine(machineName, func(mach models.Machiner) {
			mach.SetDisplayLocation(newStatus.Location)
		})
		if err != nil {
			l.Errorf("Error updating machine status: %v", err)
		}
	}
}

func (m *DisplayModel) updateIPs(machineName string, newStatus *models.DisplayStatus) {
	l := logger.Get()
	if newStatus.PublicIP != "" {
		err := m.Deployment.UpdateMachine(machineName, func(mach models.Machiner) {
			mach.SetPublicIP(newStatus.PublicIP)
		})
		if err != nil {
			l.Errorf("Error updating machine status: %v", err)
		}
	}
	if newStatus.PrivateIP != "" {
		err := m.Deployment.UpdateMachine(machineName, func(mach models.Machiner) {
			mach.SetPrivateIP(newStatus.PrivateIP)
		})
		if err != nil {
			l.Errorf("Error updating machine status: %v", err)
		}
	}
}

func (m *DisplayModel) updateElapsedTime(machineName string, newStatus *models.DisplayStatus) {
	l := logger.Get()
	if newStatus.ElapsedTime > 0 && !m.Deployment.Machines[machineName].IsComplete() {
		err := m.Deployment.UpdateMachine(machineName, func(m models.Machiner) {
			m.SetElapsedTime(newStatus.ElapsedTime)
		})
		if err != nil {
			l.Errorf("Error updating machine status: %v", err)
		}
	}
}

func (m *DisplayModel) updateOrchestratorStatus(
	machineName string,
	newStatus *models.DisplayStatus,
) {
	l := logger.Get()
	if newStatus.Orchestrator {
		err := m.Deployment.UpdateMachine(machineName, func(m models.Machiner) {
			m.SetOrchestrator(newStatus.Orchestrator)
		})
		if err != nil {
			l.Errorf("Error updating machine status: %v", err)
		}
	}
}

func (m *DisplayModel) updateServiceStates(machineName string, newStatus *models.DisplayStatus) {
	l := logger.Get()
	machine := m.Deployment.Machines[machineName]
	if machine == nil {
		l.Debugf("Machine %s not found, skipping service state update", machineName)
		return
	}

	// Convert SpotState (ProvisioningStage) to ServiceState
	var spotServiceState models.ServiceState
	switch newStatus.SpotState {
	case models.StageSpotRequested:
		spotServiceState = models.ServiceStateUpdating
	case models.StageSpotProvisioned:
		spotServiceState = models.ServiceStateSucceeded
	case models.StageSpotFailed:
		spotServiceState = models.ServiceStateFailed
	case models.StageSpotFallback:
		spotServiceState = models.ServiceStateUpdating // Transitioning to on-demand
	default:
		spotServiceState = models.ServiceStateUnknown
	}

	// Update stage based on service states and update the corresponding service state
	switch {
	// Handle spot instance specific states first
	case newStatus.SpotInstance:
		if machine.GetStage() == "" {
			newStatus.Stage = models.StageSpotRequested
		} else if machine.GetStage() == models.StageSpotFailed {
			newStatus.Stage = models.StageSpotFallback
		}
		machine.SetServiceState(models.ServiceTypeSpot.Name, spotServiceState)

	case machine.GetPublicIP() != "" &&
		(machine.GetMachineResourceState("compute.googleapis.com/Instance") == models.ResourceStateSucceeded ||
			machine.GetMachineResourceState("aws.compute/Instance") == models.ResourceStateSucceeded):
		if newStatus.SpotInstance {
			newStatus.Stage = models.StageSpotProvisioned
		} else {
			newStatus.Stage = models.StageVMProvisioned
		}
		machine.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateSucceeded)

	case newStatus.SSH != models.ServiceStateUnknown:
		if newStatus.SSH == models.ServiceStateSucceeded {
			newStatus.Stage = models.StageSSHConfigured
		} else if newStatus.SSH == models.ServiceStateFailed {
			newStatus.Stage = models.StageSSHFailed
		} else {
			newStatus.Stage = models.StageSSHConfiguring
		}
		machine.SetServiceState(models.ServiceTypeSSH.Name, newStatus.SSH)

	case newStatus.Docker != models.ServiceStateUnknown:
		if newStatus.Docker == models.ServiceStateSucceeded {
			newStatus.Stage = models.StageDockerInstalled
		} else if newStatus.Docker == models.ServiceStateFailed {
			newStatus.Stage = models.StageDockerFailed
		} else {
			newStatus.Stage = models.StageDockerInstalling
		}
		machine.SetServiceState(models.ServiceTypeDocker.Name, newStatus.Docker)

	case newStatus.CorePackages != models.ServiceStateUnknown:
		machine.SetServiceState(models.ServiceTypeCorePackages.Name, newStatus.CorePackages)

	case newStatus.Bacalhau != models.ServiceStateUnknown:
		if newStatus.Bacalhau == models.ServiceStateSucceeded {
			newStatus.Stage = models.StageBacalhauInstalled
		} else if newStatus.Bacalhau == models.ServiceStateFailed {
			newStatus.Stage = models.StageBacalhauFailed
		} else {
			newStatus.Stage = models.StageBacalhauInstalling
		}
		machine.SetServiceState(models.ServiceTypeBacalhau.Name, newStatus.Bacalhau)

	case newStatus.CustomScript != models.ServiceStateUnknown:
		if newStatus.CustomScript == models.ServiceStateSucceeded {
			newStatus.Stage = models.StageScriptCompleted
		} else if newStatus.CustomScript == models.ServiceStateFailed {
			newStatus.Stage = models.StageScriptFailed
		} else {
			newStatus.Stage = models.StageScriptExecuting
		}
		machine.SetServiceState(models.ServiceTypeScript.Name, newStatus.CustomScript)
	}

	// Update the machine's stage if a new one is set
	if newStatus.Stage != "" {
		err := m.Deployment.UpdateMachine(machineName, func(m models.Machiner) {
			m.SetStage(models.ProvisioningStage(newStatus.Stage))
		})
		if err != nil {
			l.Errorf("Error updating machine stage: %v", err)
		}
	}

	// Update status message with spot instance prefix if needed
	if newStatus.SpotInstance {
		newStatus.StatusMessage = fmt.Sprintf("[Spot] %s", newStatus.StatusMessage)
	}
}

// Init initializes the DisplayModel
func (m *DisplayModel) Init() tea.Cmd {
	return m.tickCmd()
}

// tickCmd returns a command that ticks at regular intervals
func (m *DisplayModel) tickCmd() tea.Cmd {
	return tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// getTotalRunTime calculates and returns the total run time of the deployment
func (m *DisplayModel) getTotalRunTime() time.Duration {
	if m.Deployment == nil || m.Deployment.StartTime.IsZero() {
		return 0
	}
	endTime := time.Now()
	if !m.Deployment.EndTime.IsZero() {
		endTime = m.Deployment.EndTime
	}
	return endTime.Sub(m.Deployment.StartTime)
}

// getSuccessfulOperationsCount returns the number of successful operations
func (m *DisplayModel) getSuccessfulOperationsCount() int {
	count := 0
	// TODO: Implement
	// for _, machine := range m.Deployment.GetMachines() {
	// 	if machine. == models.StatusSucceeded {
	// 		count++
	// 	}
	// }
	return count
}

// getFailedOperationsCount returns the number of failed operations
func (m *DisplayModel) getFailedOperationsCount() int {
	count := 0
	// TODO: Implement
	// for _, machine := range m.Deployment.GetMachines() {
	// 	if machine.GetStatus() == models.MachineStatusFailed {
	// 		count++
	// 	}
	// }
	return count
}

func (m *DisplayModel) getCPUUsage() float64 {
	// TODO: Implement CPU usage calculation
	return 0
}

// Update handles updates to the DisplayModel
func (m *DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		if !m.Quitting {
			return m, tea.Batch(m.tickCmd(), m.updateLogCmd(), m.applyBatchedUpdatesCmd())
		}
	case batchedUpdatesAppliedMsg:
		m.BatchUpdateTimer = nil
	case logLinesMsg:
		// Handle log lines update
	}

	// Update CPU and memory usage
	if !m.Quitting {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		m.MemoryUsage = memStats.Alloc
		m.CPUUsage = m.getCPUUsage()
	}

	return m, nil
}

// updateLogCmd returns a command that updates the log lines
func (m *DisplayModel) updateLogCmd() tea.Cmd {
	return func() tea.Msg {
		lines := logger.GetLastLines(LogLines)
		return logLinesMsg(lines)
	}
}
