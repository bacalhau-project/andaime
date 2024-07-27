package display

import (
	"bytes"
	"context"
	"os"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var testTasks []models.Status

type Display struct {
	Statuses           map[string]*models.Status
	StatusesMu         sync.RWMutex
	App                *tview.Application
	Table              *tview.Table
	TotalTasks         int
	CompletedTasks     int
	BaseHighlightColor tcell.Color
	FadeSteps          int
	StopOnce           sync.Once
	StopChan           chan struct{}
	Quit               chan struct{}
	LastTableState     [][]string
	DebugLog           logger.Logger
	Logger             logger.Logger
	LogFileName        string
	LogFile            *os.File
	LogBox             *tview.TextView
	TestMode           bool
	Ctx                context.Context
	Cancel             context.CancelFunc
	LogBuffer          *utils.CircularBuffer
	UpdatePending      bool
	UpdateMutex        sync.Mutex
	VirtualConsole     *bytes.Buffer
}

type TestDisplay struct {
	Display
	Logger *logger.Logger
}

func NewTestDisplay(totalTasks int) *TestDisplay {
	return &TestDisplay{
		Display: *NewDisplay(totalTasks),
		Logger:  logger.Get(),
	}
}

// Override the Start method for testing
func (d *TestDisplay) Start(sigChan chan os.Signal) {
	if d.Logger == nil {
		d.Logger = logger.Get()
	}
	d.Logger.Debug("Starting test display")
	d.StopChan = make(chan struct{})
	d.Quit = make(chan struct{})
	d.Statuses = make(map[string]*models.Status)
	go func() {
		<-d.StopChan
		close(d.Quit)
	}()
}

// Override the UpdateStatus method to skip tview operations
func (d *TestDisplay) UpdateStatus(status *models.Status) {
	d.Logger.Debugf("UpdateStatus called with %s", status.ID)
	d.StatusesMu.Lock()
	defer d.StatusesMu.Unlock()

	newStatus := *status // Create a copy of the status
	if _, exists := d.Statuses[newStatus.ID]; !exists {
		d.CompletedTasks++
	}
	newStatus.HighlightCycles = d.FadeSteps
	d.Statuses[newStatus.ID] = &newStatus
}

func (d *TestDisplay) Stop() {
	d.Logger.Debug("Stopping test display")
	close(d.StopChan)
}

func (d *TestDisplay) WaitForStop() {
	d.Logger.Debug("Waiting for test display to stop")
	select {
	case <-d.Quit:
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

// Global status map
var (
	GlobalStatusMap = make(map[string]*models.Status)
	StatusMutex     sync.RWMutex
)

// UpdateStatus updates the global status map
func UpdateStatus(id string, status *models.Status) {
	StatusMutex.Lock()
	defer StatusMutex.Unlock()
	GlobalStatusMap[id] = status
}

// GetStatus retrieves a status from the global map
func GetStatus(id string) *models.Status {
	StatusMutex.RLock()
	defer StatusMutex.RUnlock()
	return GlobalStatusMap[id]
}
