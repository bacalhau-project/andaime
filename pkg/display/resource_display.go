package display

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Program        *tea.Program
}

var DisplayColumns = []table.Column{
	{Title: "ID", Width: 13},
	{Title: "Type", Width: 8},
	{Title: "Location", Width: 12},
	{Title: "Status", Width: 44},
	{Title: "Time", Width: 10},
	{Title: "Public IP", Width: 15},
	{Title: "Private IP", Width: 15},
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

	if d.Visible && d.Program != nil {
		d.Program.Send(updateMsg(d.Statuses))
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

	model := initialModel(d)
	p := tea.NewProgram(model)
	d.Program = p

	go func() {
		if err := p.Start(); err != nil {
			d.Logger.Errorf("Error starting Bubble Tea program: %v", err)
		}
	}()

	d.Logger.Debug("Display started")
	d.DisplayRunning = true
}

func (d *Display) Stop() {
	d.Logger.Debug("Stopping display")
	d.Cancel() // Cancel the context
	if d.Program != nil {
		d.Program.Quit()
	}
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

func (d *Display) renderTable() string {
	d.Logger.Debug("Rendering table")

	var rows []table.Row

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
		row := table.Row{
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

	t := table.New(
		table.WithColumns(DisplayColumns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(len(rows)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return t.View()
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
