package display

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	timeoutDuration         = 5 * time.Second
	updateTimeoutDuration   = 30 * time.Second
	displayStopTimeout      = 10 * time.Second
	timeBetweenDisplayTicks = 100 * time.Millisecond
	MaxLogLines             = 8
)

type Display struct {
	Statuses       map[string]*models.Status
	StatusesMu     sync.RWMutex
	Ctx            context.Context
	Cancel         context.CancelFunc
	Logger         *logger.Logger
	Visible        bool
	DisplayRunning bool
	LastTableState [][]string
}

type DisplayColumn struct {
	Text     string
	Width    int
	DataFunc func(status models.Status) string
}

var DisplayColumns = []DisplayColumn{
	{Text: "ID", Width: 13, DataFunc: func(status models.Status) string { return status.ID }},
	{Text: "Type", Width: 8, DataFunc: func(status models.Status) string { return string(status.Type) }},
	{Text: "Location", Width: 12, DataFunc: func(status models.Status) string { return status.Location }},
	{Text: "Status", Width: 44, DataFunc: func(status models.Status) string { return status.Status }},
	{
		Text:  "Time",
		Width: 10,
		DataFunc: func(status models.Status) string {
			elapsedTime := status.ElapsedTime
			if elapsedTime == 0 {
				elapsedTime = time.Since(status.StartTime)
			}
			minutes := int(elapsedTime.Minutes())
			seconds := float64(elapsedTime.Milliseconds()%60000) / 1000.0
			if minutes < 1 && seconds < 10 {
				return fmt.Sprintf("%1.1fs", seconds)
			} else if minutes < 1 {
				return fmt.Sprintf("%01.1fs", seconds)
			}
			return fmt.Sprintf("%dm%01.1fs", minutes, seconds)
		},
	},
	{Text: "Public IP", Width: 15, DataFunc: func(status models.Status) string { return status.PublicIP }},
	{Text: "Private IP", Width: 15, DataFunc: func(status models.Status) string { return status.PrivateIP }},
}

func NewDisplay() *Display {
	ctx, cancel := context.WithCancel(context.Background())
	return &Display{
		Statuses: make(map[string]*models.Status),
		Ctx:      ctx,
		Cancel:   cancel,
		Logger:   logger.Get(),
		Visible:  true,
	}
}

func (d *Display) UpdateStatus(newStatus *models.Status) {
	if d == nil || newStatus == nil {
		d.Logger.Infof("Invalid state in UpdateStatus: d=%v, status=%v", d, newStatus)
		return
	}

	d.StatusesMu.Lock()
	defer d.StatusesMu.Unlock()
	if d.Statuses == nil {
		d.Statuses = make(map[string]*models.Status)
	}

	if _, exists := d.Statuses[newStatus.ID]; !exists {
		d.Statuses[newStatus.ID] = newStatus
	} else {
		d.Statuses[newStatus.ID] = utils.UpdateStatus(d.Statuses[newStatus.ID], newStatus)
	}

	if d.Visible {
		d.updateDisplay()
	}
}

func (d *Display) GetTableString() string {
	table := d.renderTable()
	return table.Render()
}

func (d *Display) Start() {
	d.Logger.Debug("Starting display")
	if !d.Visible {
		d.Logger.Debug("Display is not visible, skipping start")
		return
	}

	go func() {
		d.Logger.Debug("Display goroutine started")
		defer func() {
			if r := recover(); r != nil {
				d.Logger.Errorf("Panic in display goroutine: %v", r)
				d.Logger.Debugf("Stack trace:\n%s", debug.Stack())
			}
			d.Logger.Debug("Display goroutine exiting")
		}()

		displayTicker := time.NewTicker(timeBetweenDisplayTicks)
		updateTimeout := time.NewTimer(updateTimeoutDuration)
		defer updateTimeout.Stop()
		defer displayTicker.Stop()

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-displayTicker.C:
				d.updateDisplay()
			case sig := <-signalChan:
				d.Logger.Infof("Received signal: %v", sig)
				return
			case <-d.Ctx.Done():
				d.Logger.Debug("Context cancelled, stopping display")
				return
			case <-updateTimeout.C:
				d.Logger.Warn("Update timeout reached, possible hang detected")
				return
			}
		}
	}()

	d.Logger.Debug("Display started")
	d.DisplayRunning = true
}

func (d *Display) Stop() {
	d.Logger.Debug("Stopping display")
	d.Cancel() // Cancel the context

	d.Logger.Debug("Closing all registered channels")
	utils.CloseAllChannels()

	d.DisplayRunning = false
	d.Logger.Debug("Display stop process completed")
	d.DumpGoroutines()
	d.Logger.Debug("Dumped goroutines")
}

func (d *Display) WaitForStop() {
	d.Logger.Debug("Waiting for display to stop")
	timeout := time.After(displayStopTimeout)
	ticker := time.NewTicker(timeBetweenDisplayTicks)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !d.DisplayRunning {
				d.Logger.Debug("Display stopped")
				return
			}
			d.Logger.Debug("Display still running, waiting...")
		case <-timeout:
			d.Logger.Warn("Timeout waiting for display to stop")
			d.Logger.Debug("Forcing display stop")
			d.forceStop()
			return
		}
	}
}

func (d *Display) forceStop() {
	d.Logger.Debug("Force stopping display")
	d.DisplayRunning = false
	d.Logger.Debug("Display force stopped")
}

func (d *Display) updateDisplay() {
	d.Logger.Debug("Updating display")
	table := d.renderTable()
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top-left
	fmt.Println(table.Render())
}

func (d *Display) renderTable() lipgloss.Table {
	d.Logger.Debug("Rendering table")

	var rows [][]string

	// Add header row
	headerRow := make([]string, len(DisplayColumns))
	for i, col := range DisplayColumns {
		headerRow[i] = col.Text
	}
	rows = append(rows, headerRow)

	resources := make([]*models.Status, 0)
	d.StatusesMu.RLock()
	for _, status := range d.Statuses {
		resources = append(resources, status)
	}
	d.StatusesMu.RUnlock()

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].ID < resources[j].ID
	})

	for _, resource := range resources {
		row := make([]string, len(DisplayColumns))
		for i, col := range DisplayColumns {
			row[i] = col.DataFunc(*resource)
		}
		rows = append(rows, row)
	}

	d.LastTableState = rows

	table := lipgloss.NewTable().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("236")))

	for _, row := range rows {
		var rowEntries []interface{}
		for i, cell := range row {
			style := lipgloss.NewStyle().Width(DisplayColumns[i].Width).Align(lipgloss.Center)
			if i == 0 {
				rowEntries = append(rowEntries, style.Render(cell))
			} else {
				rowEntries = append(rowEntries, style.Render(cell))
			}
		}
		table.Row(rowEntries...)
	}

	d.Logger.Debugf("Table rendered with %d rows", len(rows))
	return table
}

func (d *Display) DumpGoroutines() {
	_, _ = fmt.Fprintf(&logger.GlobalLoggedBuffer, "pprof at end of executeCreateDeployment\n")
	_ = pprof.Lookup("goroutine").WriteTo(&logger.GlobalLoggedBuffer, 1)
}
