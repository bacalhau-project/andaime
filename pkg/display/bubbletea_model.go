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

// Constants
const (
	TableWidth         = 120
	LogLines           = 10
	AzureTotalSteps    = 7
	StatusLength       = 30
	TickerInterval     = 250 * time.Millisecond
	ProgressBarPadding = 2
)

// DisplayColumn represents a column in the display table
type DisplayColumn struct {
	Title       string
	Width       int
	EmojiColumn bool
}

// DisplayColumns defines the structure of the display table
var DisplayColumns = []DisplayColumn{
	{Title: "Name", Width: 10},
	{Title: "Type", Width: 6},
	{Title: "Location", Width: 16},
	{Title: "Status", Width: StatusLength},
	{Title: "Progress", Width: 20},
	{Title: "Time", Width: 8},
	{Title: "Pub IP", Width: 17},
	{Title: "Priv IP", Width: 17},
	{Title: "O", Width: 3, EmojiColumn: true},
	{Title: "S", Width: 3, EmojiColumn: true},
	{Title: "D", Width: 3, EmojiColumn: true},
	{Title: "B", Width: 3, EmojiColumn: true},
	{Title: "", Width: 3},
}

// DisplayModel represents the main display model
type DisplayModel struct {
	Deployment *models.Deployment
	TextBox    []string
	Quitting   bool
	LastUpdate time.Time
	DebugMode  bool
}

var (
	globalModelInstance *DisplayModel
	globalModelOnce     sync.Once
)

// GetGlobalModel returns the singleton instance of DisplayModel
func GetGlobalModel() *DisplayModel {
	globalModelOnce.Do(func() {
		globalModelInstance = InitialModel()
	})
	return globalModelInstance
}

// InitialModel creates and returns a new DisplayModel
func InitialModel() *DisplayModel {
	return &DisplayModel{
		Deployment: models.NewDeployment(),
		TextBox:    []string{"Resource Status Monitor"},
		LastUpdate: time.Now(),
		DebugMode:  os.Getenv("DEBUG_DISPLAY") == "1",
	}
}

// Init initializes the DisplayModel
func (m *DisplayModel) Init() tea.Cmd {
	return tickCmd()
}

// Update handles updates to the DisplayModel
func (m *DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.Quitting = true
			return m, tea.Sequence(
				tea.ExitAltScreen,
				m.printFinalTableCmd(),
				tea.Quit,
			)
		}
	case tickMsg:
		if m.Quitting {
			return m, nil
		}
		return m, tea.Batch(tickCmd(), m.updateLogCmd())
	case models.StatusUpdateMsg:
		m.UpdateStatus(msg.Status)
	case models.TimeUpdateMsg:
		m.LastUpdate = time.Now()
	case logLinesMsg:
		m.TextBox = []string(msg)
	}
	return m, nil
}

// View renders the DisplayModel
func (m *DisplayModel) View() string {
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

	tableStr := m.renderTable(headerStyle, cellStyle)
	logContent := strings.Join(logger.GetLastLines(LogLines), "\n")
	infoText := fmt.Sprintf(
		"Press 'q' or Ctrl+C to quit (Last Updated: %s)",
		m.LastUpdate.Format("15:04:05"),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		textBoxStyle.Render(logContent),
		infoStyle.Render(infoText),
	)
}

// RenderFinalTable renders the final table
func (m *DisplayModel) RenderFinalTable() string {
	return m.View()
}

// UpdateStatus updates the status of a machine
func (m *DisplayModel) UpdateStatus(status *models.DisplayStatus) {
	if status == nil || status.Name == "" {
		return
	}

	machine, found := m.findOrCreateMachine(status)
	if found || (status.Name != "" && status.Type == models.AzureResourceTypeVM) {
		m.updateMachineStatus(machine, status)
	}
}

// Helper functions

func (m *DisplayModel) renderTable(headerStyle, cellStyle lipgloss.Style) string {
	var tableStr string
	tableStr += m.renderRow(DisplayColumns, headerStyle, true)
	if m.DebugMode {
		tableStr += strings.Repeat("-", TableWidth) + "\n"
	}
	for _, machine := range m.Deployment.Machines {
		if machine.Name != "" {
			tableStr += m.renderRow(m.getMachineRowData(machine), cellStyle, false)
		}
	}
	return tableStr
}

func (m *DisplayModel) renderRow(data interface{}, baseStyle lipgloss.Style, isHeader bool) string {
	var rowStr string
	var cellData []string

	if isHeader {
		for _, col := range data.([]DisplayColumn) {
			cellData = append(cellData, col.Title)
		}
	} else {
		cellData = data.([]string)
	}

	for i, cell := range cellData {
		style := baseStyle.Copy().
			Width(DisplayColumns[i].Width).
			MaxWidth(DisplayColumns[i].Width)

		if DisplayColumns[i].EmojiColumn {
			if isHeader {
				style = style.Align(lipgloss.Center).Inline(true)
			} else {
				style = renderStyleByColumn(cell, style)
			}
		}

		renderedCell := style.Render(cell)
		if m.DebugMode {
			rowStr += fmt.Sprintf("%s[%d]", renderedCell, len(renderedCell))
		} else {
			rowStr += renderedCell
		}
	}
	return strings.TrimRight(rowStr, " ") + "\n"
}

func (m *DisplayModel) getMachineRowData(machine models.Machine) []string {
	elapsedTime := time.Since(machine.StartTime).Truncate(TickerInterval)
	progressBar := renderProgressBar(
		machine.Progress,
		AzureTotalSteps,
		DisplayColumns[4].Width-ProgressBarPadding,
	)
	orchString := models.DisplayEmojiWorkerNode
	if machine.Orchestrator {
		orchString = models.DisplayEmojiOrchestratorNode
	}

	return []string{
		machine.Name,
		machine.Type.ShortResourceName,
		machine.Location,
		machine.Status,
		progressBar,
		formatElapsedTime(elapsedTime),
		machine.PublicIP,
		machine.PrivateIP,
		orchString,
		machine.SSH,
		machine.Docker,
		machine.Bacalhau,
	}
}

func (m *DisplayModel) findOrCreateMachine(status *models.DisplayStatus) (*models.Machine, bool) {
	for i, machine := range m.Deployment.Machines {
		if machine.Name == status.Name {
			return &m.Deployment.Machines[i], true
		}
	}

	if status.Name != "" && status.Type == models.AzureResourceTypeVM {
		newMachine := models.Machine{
			Name:      status.Name,
			Type:      status.Type,
			Location:  status.Location,
			Status:    status.Status,
			StartTime: time.Now(),
		}
		m.Deployment.Machines = append(m.Deployment.Machines, newMachine)
		return &m.Deployment.Machines[len(m.Deployment.Machines)-1], false
	}

	return nil, false
}

func (m *DisplayModel) updateMachineStatus(machine *models.Machine, status *models.DisplayStatus) {
	l := logger.Get()
	if status.Status != "" {
		trimmedStatus := strings.TrimSpace(status.Status)
		if len(trimmedStatus) > StatusLength-3 {
			l.Debugf("Status too long, truncating: '%s'", trimmedStatus)
			machine.Status = trimmedStatus[:StatusLength-3] + "…"
		} else {
			machine.Status = fmt.Sprintf("%-*s", StatusLength, trimmedStatus)
		}
	}

	if status.Location != "" {
		machine.Location = status.Location
	}
	if status.PublicIP != "" {
		machine.PublicIP = status.PublicIP
	}
	if status.PrivateIP != "" {
		machine.PrivateIP = status.PrivateIP
	}
	if status.Progress != 0 {
		machine.Progress = status.Progress
	}
	if status.ElapsedTime > 0 {
		machine.ElapsedTime = status.ElapsedTime
	}
	if status.Orchestrator {
		machine.Orchestrator = status.Orchestrator
	}
	if status.SSH != "" {
		machine.SSH = status.SSH
	}
	if status.Docker != "" {
		machine.Docker = status.Docker
	}
	if status.Bacalhau != "" {
		machine.Bacalhau = status.Bacalhau
	}
}

func renderStyleByColumn(status string, style lipgloss.Style) lipgloss.Style {
	style = style.Bold(true).Align(lipgloss.Center)
	switch status {
	case models.DisplayEmojiSuccess:
		style = style.Foreground(lipgloss.Color("#00c413"))
	case models.DisplayEmojiWaiting:
		style = style.Foreground(lipgloss.Color("#69acdb"))
	case models.DisplayEmojiNotStarted:
		style = style.Foreground(lipgloss.Color("#2e2d2d"))
	case models.DisplayEmojiFailed:
		style = style.Foreground(lipgloss.Color("#ff0000"))
	}
	return style
}

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

func formatElapsedTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds()/100) % 10

	if minutes > 0 {
		return fmt.Sprintf("%dm%02d.%ds", minutes, seconds, tenths)
	}
	return fmt.Sprintf("%2d.%ds", seconds, tenths)
}

// Commands and messages

func (m *DisplayModel) printFinalTableCmd() tea.Cmd {
	return func() tea.Msg {
		fmt.Print("\n" + m.RenderFinalTable() + "\n")
		return nil
	}
}

func (m *DisplayModel) updateLogCmd() tea.Cmd {
	return func() tea.Msg {
		return logLinesMsg(logger.GetLastLines(LogLines))
	}
}

type tickMsg time.Time
type logLinesMsg []string

func tickCmd() tea.Cmd {
	return tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Deprecated functions

// SetGlobalModel is deprecated and will be removed in future versions
// Use GetGlobalModel() instead
func SetGlobalModel(m *DisplayModel) {
	logger.Get().
		Warnf("SetGlobalModel is deprecated and will be removed in future versions. Use GetGlobalModel() instead.")
	globalModelInstance = m
}
