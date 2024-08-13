package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type DisplayColumn struct {
	Title string
	Width int
}

var DisplayColumns = []DisplayColumn{
	{Title: "ID", Width: 13},
	{Title: "Type", Width: 8},
	{Title: "Location", Width: 12},
	{Title: "Status", Width: 20},
	{Title: "Progress", Width: 20},
	{Title: "Time", Width: 10},
	{Title: "Public IP", Width: 15},
	{Title: "Private IP", Width: 15},
}

type DisplayModel struct {
	Deployment *models.Deployment
	TextBox    string
	Quitting   bool
}

func InitialModel() *DisplayModel {
	return &DisplayModel{
		Deployment: models.NewDeployment(),
		TextBox:    "Resource Status Monitor",
	}
}

func (m *DisplayModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.ClearScreen,
	)
}

func (m *DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		if m.Quitting {
			return m, nil
		}
		m.TextBox = fmt.Sprintf("Last Updated: %s", time.Now().Format("15:04:05"))
		allFinished := true
		for _, machine := range m.Deployment.Machines {
			if machine.Status != "Successfully Deployed" && machine.Status != "Failed" {
				allFinished = false
				break
			}
		}
		if allFinished {
			m.Quitting = true
			return m, tea.Quit
		}
		return m, tickCmd()
	case models.StatusUpdateMsg:
		m.Deployment.UpdateStatus(msg.Status)
		return m, nil
	}
	return m, nil
}

func (m *DisplayModel) View() string {
	tableStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))
	cellStyle := lipgloss.NewStyle().
		PaddingLeft(1)
	textBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(70)
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	var tableStr string

	// Render headers
	var headerRow string
	for _, col := range DisplayColumns {
		headerRow += headerStyle.Width(col.Width).MaxWidth(col.Width).Render(col.Title)
	}
	tableStr += headerRow + "\n"

	// Render rows
	for _, machine := range m.Deployment.Machines {
		var rowStr string
		elapsedTime := time.Since(machine.StartTime).Truncate(time.Second).String()
		progressBar := renderProgressBar(0, 100, DisplayColumns[4].Width-2) // Placeholder progress

		rowData := []string{
			machine.ID,
			machine.Type,
			machine.Location,
			machine.Status,
			progressBar,
			elapsedTime,
			machine.PublicIP,
			machine.PrivateIP,
		}

		for i, cell := range rowData {
			rowStr += cellStyle.Copy().
				Width(DisplayColumns[i].Width).
				MaxWidth(DisplayColumns[i].Width).
				Render(cell)
		}
		tableStr += rowStr + "\n"
	}

	infoText := infoStyle.Render("Press 'q' or Ctrl+C to quit")

	output := lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		textBoxStyle.Render(m.TextBox),
		"",
		infoText,
	)

	return output
}

func renderProgressBar(progress, total, width int) string {
	if total == 0 {
		return ""
	}
	filledWidth := int(float64(progress) / float64(total) * float64(width))
	emptyWidth := width - filledWidth

	filled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Render(strings.Repeat("█", filledWidth))
	empty := lipgloss.NewStyle().
		Foreground(lipgloss.Color("237")).
		Render(strings.Repeat("█", emptyWidth))

	return filled + empty
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
