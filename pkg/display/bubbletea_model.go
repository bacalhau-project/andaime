package display

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	globalModelInstance *DisplayModel
	globalModelOnce     sync.Once

	azureTotalSteps = 7
)

type DisplayColumn struct {
	Title string
	Width int
}

var DisplayColumns = []DisplayColumn{
	{Title: "Name", Width: 13},
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
	TextBox    []string
	Quitting   bool
}

// GetGlobalProgram returns the singleton instance of GlobalProgram
func GetGlobalModel() *DisplayModel {
	if globalModelInstance != nil {
		return globalModelInstance
	}
	globalModelOnce.Do(func() {
		globalModelInstance = InitialModel()
	})
	return globalModelInstance
}

func SetGlobalModel(m *DisplayModel) {
	globalModelInstance = m
}

func InitialModel() *DisplayModel {
	return &DisplayModel{
		Deployment: models.NewDeployment(),
		TextBox:    []string{"Resource Status Monitor"},
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
		return m, tea.Batch(
			tickCmd(),
			m.updateLogCmd(),
		)
	case models.StatusUpdateMsg:
		m.updateStatus(msg.Status)
		return m, nil
	case logLinesMsg:
		m.TextBox = []string(msg)
		return m, nil
	}
	return m, nil
}

func (m *DisplayModel) updateStatus(status *models.Status) {
	found := false
	for i, machine := range m.Deployment.Machines {
		if machine.Name == status.Name {
			if status.Status != "" {
				m.Deployment.Machines[i].Status = status.Status
			}
			if status.Location != "" {
				m.Deployment.Machines[i].Location = status.Location
			}
			if status.PublicIP != "" {
				m.Deployment.Machines[i].PublicIP = status.PublicIP
			}
			if status.PrivateIP != "" {
				m.Deployment.Machines[i].PrivateIP = status.PrivateIP
			}
			if status.Progress != 0 {
				m.Deployment.Machines[i].Progress = status.Progress
			}
			found = true
			break
		}
	}
	if !found {
		m.Deployment.Machines = append(m.Deployment.Machines, models.Machine{
			Name:     status.Name,
			Type:     string(status.Type),
			Location: status.Location,
			Status:   status.Status,
		})
	}
}

func (m *DisplayModel) View() string {
	tableWidth := 0
	for _, col := range DisplayColumns {
		tableWidth += col.Width
	}

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
		Height(10).
		Width(tableWidth)
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
		progressBar := renderProgressBar(
			machine.Progress,
			machine.ProgressFinish,
			DisplayColumns[4].Width-2,
		)

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
			rowStr += cellStyle.
				Width(DisplayColumns[i].Width).
				MaxWidth(DisplayColumns[i].Width).
				Render(cell)
		}
		tableStr += rowStr + "\n"
	}

	infoText := infoStyle.Render("Press 'q' or Ctrl+C to quit")

	var textBoxContent string
	if len(m.TextBox) > 0 {
		textBoxContent = strings.Join(m.TextBox, "\n")
	}

	lastUpdated := fmt.Sprintf("Last Updated: %s", time.Now().Format("15:04:05"))
	
	output := lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		infoText,
		"",
		textBoxStyle.Render(textBoxContent),
		"",
		lastUpdated,
	)

	return output
}

func (m *DisplayModel) updateLogCmd() tea.Cmd {
	return func() tea.Msg {
		logLines := logger.GetLastLines(8)
		return logLinesMsg(logLines)
	}
}

type logLinesMsg []string

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
