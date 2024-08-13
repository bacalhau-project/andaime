package display

import (
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	table   table.Model
	display *Display
}

type updateMsg map[string]*models.Status

func initialModel(d *Display) model {
	return model{
		table:   table.New(table.WithColumns(DisplayColumns)),
		display: d,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.display.Stop()
			return m, tea.Quit
		}
	case updateMsg:
		m.display.Statuses = msg
		m.table.SetRows(m.getRows())
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.display.renderTable()
}

func (m model) getRows() []table.Row {
	var rows []table.Row
	for _, status := range m.display.Statuses {
		row := table.Row{
			status.ID,
			string(status.Type),
			status.Location,
			status.Status,
			formatElapsedTime(status.ElapsedTime, status.StartTime),
			status.PublicIP,
			status.PrivateIP,
		}
		rows = append(rows, row)
	}
	return rows
}
