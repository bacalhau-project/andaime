package display

import (
	"context"
	"os"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var testTasks []models.Status

type Display struct {
	Statuses   map[string]*models.Status
	StatusesMu sync.RWMutex
	UpdateChan chan struct{}
	UpdateMu   sync.Mutex
	StopChan   chan struct{}

	UpdatePending  bool
	DisplayRunning bool

	App        *tview.Application
	Table      *tview.Table
	LogBox     *tview.TextView
	Ctx        context.Context
	Cancel     context.CancelFunc
	Logger     *logger.Logger
	updateChan chan struct{}

	FadeSteps          int
	BaseHighlightColor tcell.Color

	Quit chan struct{}

	LastTableState [][]string
}

func (d *Display) Close() {
	d.App.Stop()
}

type TestDisplay struct {
	Display
	Logger *logger.Logger
}

func NewTestDisplay(totalTasks int) *TestDisplay {
	return &TestDisplay{
		Display: *NewDisplay(),
		Logger:  logger.Get(),
	}
}

// Override the Start method for testing
func (d *TestDisplay) Start(sigChan chan os.Signal) {
	if d.Logger == nil {
		d.Logger = logger.Get()
	}
	d.Logger.Debug("Starting test display")
	d.Statuses = make(map[string]*models.Status)
}

// Override the UpdateStatus method to skip tview operations
func (d *TestDisplay) UpdateStatus(status *models.Status) {
	d.Logger.Debugf("UpdateStatus called with %s", status.ID)
	d.StatusesMu.Lock()
	defer d.StatusesMu.Unlock()

	newStatus := *status // Create a copy of the status
	d.Statuses[newStatus.ID] = &newStatus
}

func (d *TestDisplay) Stop() {
	d.Logger.Debug("Stopping test display")
}

func (d *TestDisplay) WaitForStop() {
	d.Logger.Debug("Waiting for test display to stop")
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

// GetStatus retrieves a status from the global map
func GetStatus(id string) *models.Status {
	StatusMutex.RLock()
	defer StatusMutex.RUnlock()
	return GlobalStatusMap[id]
}
