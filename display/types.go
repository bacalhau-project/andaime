package display

import (
	"os"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/logger"
	"github.com/gdamore/tcell/v2"
)

type TestDisplayType struct {
	statuses           map[string]*Status
	statusesMu         sync.RWMutex
	totalTasks         int
	completedTasks     int
	fadeSteps          int
	baseHighlightColor tcell.Color
	stopChan           chan struct{}
	quit               chan struct{}
	testMode           bool
	DebugLog           logger.Logger
}

func NewTestDisplay(totalTasks int) *TestDisplayType {
	d := &TestDisplayType{
		statuses:           make(map[string]*Status),
		totalTasks:         totalTasks,
		baseHighlightColor: HighlightColor,
		fadeSteps:          NumberOfCyclesToHighlight,
		stopChan:           make(chan struct{}),
		quit:               make(chan struct{}),
		DebugLog:           *logger.Get(),
		testMode:           true,
	}
	return d
}

// Override the Start method for testing
func (d *TestDisplayType) Start(sigChan chan os.Signal) {
	d.DebugLog.Debug("Starting test display")
	go func() {
		<-d.stopChan
		close(d.quit)
	}()
}

// Override the UpdateStatus method to skip tview operations
func (d *TestDisplayType) UpdateStatus(status *Status) {
	d.DebugLog.Debugf("UpdateStatus called with %s", status.ID)
	d.statusesMu.Lock()
	defer d.statusesMu.Unlock()

	newStatus := *status // Create a copy of the status
	if _, exists := d.statuses[newStatus.ID]; !exists {
		d.completedTasks++
	}
	newStatus.HighlightCycles = d.fadeSteps
	d.statuses[newStatus.ID] = &newStatus
}

func (d *TestDisplayType) Stop() {
	d.DebugLog.Debug("Stopping test display")
	close(d.stopChan)
}

func (d *TestDisplayType) WaitForStop() {
	d.DebugLog.Debug("Waiting for test display to stop")
	select {
	case <-d.quit:
		d.DebugLog.Debug("Test display stopped")
	case <-time.After(5 * time.Second): //nolint:gomnd
		d.DebugLog.Debug("Timeout waiting for test display to stop")
	}
}

//nolint:unused
func (d *TestDisplayType) getHighlightColor(cycles int) tcell.Color {
	if cycles <= 0 {
		return tcell.ColorDefault
	}

	return HighlightColor
}
