package display

import (
	"fmt"
	"os"
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

	TableWidth         = 120
	LogLines           = 10
	AzureTotalSteps    = 7
	StatusLength       = 30
	TickerInterval     = 250 * time.Millisecond
	ProgressBarPadding = 2
)

type DisplayColumn struct {
	Title       string
	Width       int
	EmojiColumn bool
}

var DisplayColumns = []DisplayColumn{
	{Title: "Name", Width: 10, EmojiColumn: false},
	{Title: "Type", Width: 6, EmojiColumn: false},
	{Title: "Location", Width: 15, EmojiColumn: false},
	{Title: "Status", Width: StatusLength, EmojiColumn: false},
	{Title: "Progress", Width: 20, EmojiColumn: false},
	{Title: "Time", Width: 10, EmojiColumn: false},
	{Title: "Pub IP", Width: 15, EmojiColumn: false},
	{Title: "Priv IP", Width: 15, EmojiColumn: false},
	{Title: "O", Width: 3, EmojiColumn: true},
	{Title: "S", Width: 3, EmojiColumn: true},
	{Title: "D", Width: 3, EmojiColumn: true},
	{Title: "B", Width: 3, EmojiColumn: true},
}

type DisplayModel struct {
	Deployment *models.Deployment
	TextBox    []string
	Quitting   bool
	LastUpdate time.Time
	DebugMode  bool
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
	debugMode := os.Getenv("DEBUG_DISPLAY") == "1"
	return &DisplayModel{
		Deployment: models.NewDeployment(),
		TextBox:    []string{"Resource Status Monitor"},
		LastUpdate: time.Now(),
		DebugMode:  debugMode,
	}
}

func (m *DisplayModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func (m *DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Sequence(
				tea.ExitAltScreen,
				tea.Println("\nFinal Table:"),
				tea.Printf(m.RenderFinalTable()),
				tea.Quit,
			)
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

func (m *DisplayModel) RenderFinalTable() string {
	tableStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	return tableStyle.Render(m.renderTable())
}

func (m *DisplayModel) renderTable() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(0, 1)
	cellStyle := lipgloss.NewStyle().
		PaddingLeft(1)

	var tableStr string

	// Render headers
	var headerRow string
	for _, col := range DisplayColumns {
		style := headerStyle.
			Width(col.Width).
			MaxWidth(col.Width)

		if col.EmojiColumn {
			style = style.Align(lipgloss.Center).Inline(true)
		}

		headerRow += style.Render(col.Title)
	}
	tableStr += strings.TrimRight(headerRow, " ") + "\n"

	// Render rows
	for _, machine := range m.Deployment.Machines {
		if machine.Name == "" || machine.Type == "" {
			continue // Skip invalid entries
		}

		var rowStr string
		elapsedTime := time.Since(machine.StartTime).Truncate(TickerInterval)
		elapsedTimeStr := fmt.Sprintf("%7s", formatElapsedTime(elapsedTime))
		progressBar := renderProgressBar(
			machine.Progress,
			AzureTotalSteps,
			DisplayColumns[4].Width-ProgressBarPadding,
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
				style = renderStyleByColumn(cell, style)
			}

			rowStr += style.Render(cell)
		}
		tableStr += strings.TrimRight(rowStr, " ") + "\n"
	}

	return tableStr
}

func (m *DisplayModel) updateStatus(status *models.Status) {
	l := logger.Get()
	if status == nil || status.Name == "" {
		return // Ignore invalid status updates
	}

	found := false
	for i, machine := range m.Deployment.Machines {
		if machine.Name == status.Name {
			if status.Status != "" {
				trimmedStatus := strings.TrimSpace(status.Status)
				if len(trimmedStatus) > StatusLength-3 {
					l.Debugf("Status too long, truncating: '%s'", trimmedStatus)
					m.Deployment.Machines[i].Status = trimmedStatus[:StatusLength-3] + "…"
				} else {
					m.Deployment.Machines[i].Status = fmt.Sprintf("%-*s", StatusLength, trimmedStatus)
				}
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
			if status.Orchestrator {
				m.Deployment.Machines[i].Orchestrator = status.Orchestrator
			}
			if status.SSH != "" {
				m.Deployment.Machines[i].SSH = status.SSH
			}
			if status.Docker != "" {
				m.Deployment.Machines[i].Docker = status.Docker
			}
			if status.Bacalhau != "" {
				m.Deployment.Machines[i].Bacalhau = status.Bacalhau
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
	// l := logger.Get()

	// l.Debugf("Table Width: %d", tableWidth)
	tableStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(0, 1)
	cellStyle := lipgloss.NewStyle().
		PaddingLeft(1)
	textBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Height(LogLines).
		Width(TableWidth)
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	var tableStr string

	// Render headers
	var headerRow string
	for _, col := range DisplayColumns {
		style := headerStyle.
			Width(col.Width).
			MaxWidth(col.Width)

		if col.EmojiColumn {
			style = style.Align(lipgloss.Center).Inline(true)
		}

		renderedTitle := style.Render(col.Title)

		if m.DebugMode {
			headerRow += fmt.Sprintf("%s[%d]", renderedTitle, len(renderedTitle))
		} else {
			headerRow += renderedTitle
		}
	}
	tableStr += strings.TrimRight(headerRow, " ") + "\n"

	if m.DebugMode {
		// Add a ruler for easier width measurement
		ruler := strings.Repeat("-", TableWidth)
		tableStr += ruler + "\n"
	}

	// Render rows
	for _, machine := range m.Deployment.Machines {
		if machine.Name == "" || machine.Type == "" {
			continue // Skip invalid entries
		}

		var rowStr string
		elapsedTime := time.Since(machine.StartTime).Truncate(TickerInterval)
		elapsedTimeStr := fmt.Sprintf("%7s", formatElapsedTime(elapsedTime))
		progressBar := renderProgressBar(
			machine.Progress,
			AzureTotalSteps,
			DisplayColumns[4].Width-ProgressBarPadding,
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
			//nolint:gosec
			// randomColor := lipgloss.Color(fmt.Sprintf("#%06x", rand.Intn(0xFFFFFF)))
			style := cellStyle.
				Width(DisplayColumns[i].Width).
				MaxWidth(DisplayColumns[i].Width).
				Background(lipgloss.Color("235"))
			if DisplayColumns[i].EmojiColumn {
				style = renderStyleByColumn(cell, style)
			}

			renderedCell := style.Render(cell)
			// l.Debugf("Cell Value: %v", style.Value())
			if m.DebugMode {
				rowStr += fmt.Sprintf("%s[%d]", renderedCell, len(renderedCell))
			} else {
				rowStr += renderedCell
			}
		}
		tableStr += strings.TrimRight(rowStr, " ") + "\n"
	}

	lastUpdated := m.LastUpdate.Format("15:04:05")
	infoText := infoStyle.Render(
		fmt.Sprintf("Press 'q' or Ctrl+C to quit (Last Updated: %s)", lastUpdated),
	)

	logLines := logger.GetLastLines(LogLines)
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

func renderStyleByColumn(status string, style lipgloss.Style) lipgloss.Style {
	l := logger.Get()
	l.Debugf("Status: %s", status)
	style = style.Bold(true).Align(lipgloss.Center)
	switch status {
	case models.DisplayEmojiSuccess:
		style = style.Foreground(lipgloss.Color("#07590f"))
	case models.DisplayEmojiWaiting:
		style = style.Foreground(lipgloss.Color("#69acdb"))
	case models.DisplayEmojiNotStarted:
		style = style.Foreground(lipgloss.Color("#2e2d2d"))
	case models.DisplayEmojiFailed:
		style = style.Foreground(lipgloss.Color("#ff0000")) // Bright red color
	}
	return style
}

func (m *DisplayModel) updateLogCmd() tea.Cmd {
	return func() tea.Msg {
		logLines := logger.GetLastLines(LogLines)
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
	return tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatElapsedTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60         //nolint:gomnd
	tenths := int(d.Milliseconds()/100) % 10 //nolint:gomnd

	if minutes > 0 {
		return fmt.Sprintf("%dm%02d.%ds", minutes, seconds, tenths)
	}
	return fmt.Sprintf("%2d.%ds", seconds, tenths)
}
