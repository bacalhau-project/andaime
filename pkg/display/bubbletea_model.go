package display

import (
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lgTable "github.com/charmbracelet/lipgloss/table"
)

type model struct {
	Statuses map[string]models.Status
}

type DisplayColumn struct {
	Title string
	Width int
}

var DisplayColumns = []DisplayColumn{
	{Title: "ID", Width: 13},
	{Title: "Type", Width: 8},
	{Title: "Location", Width: 12},
	{Title: "Status", Width: 44},
	{Title: "Time", Width: 10},
	{Title: "Public IP", Width: 15},
	{Title: "Private IP", Width: 15},
}

type UpdateMsg models.Status

var HeaderStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("15")).
	Background(lipgloss.Color("4")).
	Bold(true).
	Width(80).
	Align(lipgloss.Center)

var OddRowStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("7")).
	Background(lipgloss.Color("0")).
	Padding(2)

var EvenRowStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("5")).
	Background(lipgloss.Color("1")).
	Padding(2)

func initialModel(d *Display) model {
	all_column_titles := []string{}
	for _, col := range DisplayColumns {
		all_column_titles = append(all_column_titles, col.Title)
	}

	return model{
		Statuses: map[string]models.Status{},
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
			return m, tea.Quit
		}
	case UpdateMsg:
		status := m.Statuses[msg.ID]
		status.ID = msg.ID
		status.Type = msg.Type
		status.Status = msg.Status
		status.ElapsedTime = time.Since(status.StartTime)
		status.PublicIP = msg.PublicIP
		status.PrivateIP = msg.PrivateIP

		m.Statuses[msg.ID] = status
	}
	return m, cmd
}

func (m model) View() string {

}

func (m model) getRows() lgTable.Data {
	var rows []lgTable.Table
	for _, status := range dsm {
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
