package display

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
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
	return d.renderTable()
}

func (d *Display) Start() {
	d.Logger.Debug("Starting display")
	if !d.Visible {
		d.Logger.Debug("Display is not visible, skipping start")
		return
	}

	go func() {
		d.Logger.Debug("Display goroutine started")
		defer d.Logger.Debug("Display goroutine exiting")

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d.updateDisplay()
			case <-d.Ctx.Done():
				d.Logger.Debug("Context cancelled, stopping display")
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
	d.DisplayRunning = false
	d.Logger.Debug("Display stop process completed")
}

func (d *Display) WaitForStop() {
	d.Logger.Debug("Waiting for display to stop")
	for d.DisplayRunning {
		time.Sleep(100 * time.Millisecond)
	}
	d.Logger.Debug("Display stopped")
}

func (d *Display) updateDisplay() {
	d.Logger.Debug("Updating display")
	table := d.renderTable()
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top-left
	fmt.Println(table)
}

func (d *Display) renderTable() string {
	d.Logger.Debug("Rendering table")

	var rows [][]string

	// Add header row
	headerRow := make([]string, len(DisplayColumns))
	for i, col := range DisplayColumns {
		headerRow[i] = col.Title
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
		row := []string{
			resource.ID,
			string(resource.Type),
			resource.Location,
			resource.Status,
			formatElapsedTime(resource.ElapsedTime, resource.StartTime),
			resource.PublicIP,
			resource.PrivateIP,
		}
		rows = append(rows, row)
	}

	d.LastTableState = rows

	var sb strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			sb.WriteString(fmt.Sprintf("%-*s", DisplayColumns[i].Width, cell))
			if i < len(row)-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString("\n")
	}

	d.Logger.Debugf("Table rendered with %d rows", len(rows))
	return sb.String()
}

func formatElapsedTime(elapsedTime time.Duration, startTime time.Time) string {
	if elapsedTime == 0 {
		elapsedTime = time.Since(startTime)
	}
	minutes := int(elapsedTime.Minutes())
	seconds := float64(elapsedTime.Milliseconds()%60000) / 1000.0
	if minutes < 1 && seconds < 10 {
		return fmt.Sprintf("%1.1fs", seconds)
	} else if minutes < 1 {
		return fmt.Sprintf("%01.1fs", seconds)
	}
	return fmt.Sprintf("%dm%01.1fs", minutes, seconds)
}
