package display

import (
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

var testTasks []Status

type TestDisplay struct {
	*Display
	Logger *logger.Logger
}

func NewTestDisplay(totalTasks int) *TestDisplay {
	return &TestDisplay{
		Display: NewDisplay(totalTasks),
		Logger:  logger.Get(),
	}
}

// Override the Start method for testing
func (d *TestDisplay) Start(sigChan chan os.Signal) {
	if d.Logger == nil {
		d.Logger = logger.Get()
	}
	d.Logger.Debug("Starting test display")
	go func() {
		<-d.stopChan
		close(d.quit)
	}()
}

// Override the UpdateStatus method to skip tview operations
func (d *TestDisplay) UpdateStatus(status *Status) {
	d.Logger.Debugf("UpdateStatus called with %s", status.ID)
	d.statusesMu.Lock()
	defer d.statusesMu.Unlock()

	newStatus := *status // Create a copy of the status
	if _, exists := d.statuses[newStatus.ID]; !exists {
		d.completedTasks++
	}
	newStatus.HighlightCycles = d.fadeSteps
	d.statuses[newStatus.ID] = &newStatus
}

func (d *TestDisplay) Stop() {
	d.Logger.Debug("Stopping test display")
	close(d.stopChan)
}

func (d *TestDisplay) WaitForStop() {
	d.Logger.Debug("Waiting for test display to stop")
	select {
	case <-d.quit:
		d.Logger.Debug("Test display stopped")
	case <-time.After(5 * time.Second): //nolint:gomnd
		d.Logger.Debug("Timeout waiting for test display to stop")
	}
}

//nolint:unused
func (d *TestDisplay) GetHighlightColor(cycles int) tcell.Color {
	if cycles <= 0 {
		return tcell.ColorDefault
	}

	return HighlightColor
}
