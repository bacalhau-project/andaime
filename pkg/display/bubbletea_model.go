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

	AzureTotalSteps = 7
)

type DisplayColumn struct {
	Title       string
	Width       int
	EmojiColumn bool
}

var DisplayColumns = []DisplayColumn{
	{Title: "Name", Width: 10, EmojiColumn: false},
	{Title: "Type", Width: 8, EmojiColumn: false},
	{Title: "Location", Width: 12, EmojiColumn: false},
	{Title: "Status", Width: 40, EmojiColumn: false},
	{Title: "Progress", Width: 24, EmojiColumn: false},
	{Title: "Time", Width: 10, EmojiColumn: false},
	{Title: "Pub IP", Width: 18, EmojiColumn: false},
	{Title: "Priv IP", Width: 18, EmojiColumn: false},
	{Title: models.DisplayEmojiOrchestrator, Width: 3, EmojiColumn: true},
	{Title: models.DisplayEmojiSSH, Width: 3, EmojiColumn: true},
	{Title: models.DisplayEmojiDocker, Width: 3, EmojiColumn: true},
	{Title: models.DisplayEmojiBacalhau, Width: 3, EmojiColumn: true},
}

type DisplayModel struct {
	Deployment *models.Deployment
	TextBox    []string
	Quitting   bool
	LastUpdate time.Time
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
		LastUpdate: time.Now(),
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
		return m, tea.Batch(
			tickCmd(),
			m.updateLogCmd(),
		)
	case models.StatusUpdateMsg:
		m.updateStatus(msg.Status)
		return m, nil
	case models.TimeUpdateMsg:
		m.LastUpdate = time.Now()
		return m, nil
	case logLinesMsg:
		m.TextBox = []string(msg)
		return m, nil
	}
	return m, nil
}

func (m *DisplayModel) updateStatus(status *models.Status) {
	if status == nil || status.Name == "" {
		return // Ignore invalid status updates
	}

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
			if status.ElapsedTime > 0 {
				m.Deployment.Machines[i].ElapsedTime = status.ElapsedTime
			}
			found = true
			break
		}
	}
	if !found && status.Name != "" && string(status.Type) != "" {
		m.Deployment.Machines = append(m.Deployment.Machines, models.Machine{
			Name:      status.Name,
			Type:      string(status.Type),
			Location:  status.Location,
			Status:    status.Status,
			StartTime: time.Now(),
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
		Foreground(lipgloss.Color("39")).
		PaddingLeft(1)
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

	// Render headers with debug info
	var headerRow string
	for _, col := range DisplayColumns {
		style := headerStyle.Width(col.Width).MaxWidth(col.Width)
		if col.EmojiColumn {
			style = style.UnsetPadding()
		}
		renderedTitle := style.Render(col.Title)
		headerRow += fmt.Sprintf("%s[%d]", renderedTitle, len(renderedTitle))
	}
	tableStr += headerRow + "\n"

	// Add a ruler for easier width measurement
	ruler := strings.Repeat("-", tableWidth)
	tableStr += ruler + "\n"

	// Render rows
	for _, machine := range m.Deployment.Machines {
		if machine.Name == "" || machine.Type == "" {
			continue // Skip invalid entries
		}

		var rowStr string
		elapsedTime := time.Since(machine.StartTime).Truncate(100 * time.Millisecond)
		elapsedTimeStr := fmt.Sprintf("%7s", formatElapsedTime(elapsedTime))
		progressBar := renderProgressBar(
			machine.Progress,
			AzureTotalSteps,
			DisplayColumns[4].Width-2,
		)

		orchString := models.DisplayEmojiWorkerNode
		if machine.Orchestrator {
			orchString = models.DisplayEmojiOrchestratorNode
		}
		rowData := []string{
			machine.Name,
			machine.Type,
			machine.Location,
			machine.Status,
			progressBar,
			elapsedTimeStr,
			machine.PublicIP,
			machine.PrivateIP,
			orchString,
			machine.SSH,
			machine.Docker,
			machine.Bacalhau,
		}

		for i, cell := range rowData {
			style := cellStyle.
				Width(DisplayColumns[i].Width).
				MaxWidth(DisplayColumns[i].Width)
			if DisplayColumns[i].EmojiColumn {
				style = style.UnsetPadding()
				style = style.UnsetMargins()
			}
			renderedCell := style.Render(cell)
			rowStr += fmt.Sprintf("%s[%d]", renderedCell, len(renderedCell))
		}
		tableStr += rowStr + "\n"
	}

	lastUpdated := m.LastUpdate.Format("15:04:05")
	infoText := infoStyle.Render(
		fmt.Sprintf("Press 'q' or Ctrl+C to quit (Last Updated: %s)", lastUpdated),
	)

	logLines := logger.GetLastLines(8)
	textBoxContent := strings.Join(logLines, "\n")

	output := lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		infoText,
		"",
		textBoxStyle.Render(textBoxContent),
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
	filledWidth := progress * width / total
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
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatElapsedTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds() / 100) % 10

	if minutes > 0 {
		return fmt.Sprintf("%dm%02d.%ds", minutes, seconds, tenths)
	}
	return fmt.Sprintf("%2d.%ds", seconds, tenths)
}
