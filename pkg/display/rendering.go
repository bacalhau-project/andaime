// File: rendering.go

package display

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// Constants for rendering
const (
	StatusLength         = 40
	EmojiColumnWidth     = 2
	TickerInterval       = 100 * time.Millisecond
	ProgressBarPadding   = 2
	secondsPerMinute     = 60
	MillisecondsPerTenth = 100
	TenthsPerSecond      = 10
)

// DisplayColumn represents a column in the display table
type DisplayColumn struct {
	Title       string
	Width       int
	Height      int
	EmojiColumn bool
}

// DisplayColumns defines the structure of the display table
var DisplayColumns = []DisplayColumn{
	{Title: "Name", Width: 10},
	{Title: "Type", Width: 6},
	{Title: "Location", Width: 16},
	{Title: "Status", Width: StatusLength},
	{Title: "Progress", Width: 20},
	{Title: "Time", Width: 10},
	{Title: "Pub IP", Width: 20},
	{Title: "Priv IP", Width: 20},
	{Title: models.DisplayTextOrchestrator, Width: EmojiColumnWidth, EmojiColumn: true},
	{Title: models.DisplayTextSSH, Width: EmojiColumnWidth, EmojiColumn: true},
	{Title: models.DisplayTextDocker, Width: EmojiColumnWidth, EmojiColumn: true},
	{Title: models.DisplayTextBacalhau, Width: EmojiColumnWidth, EmojiColumn: true},
	{Title: models.DisplayTextCustomScript, Width: EmojiColumnWidth, EmojiColumn: true},
	{Title: "", Width: 1},
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
		PaddingLeft(1).
		AlignVertical(lipgloss.Center)
	textBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Height(LogLines).
		Width(AggregateColumnWidths())
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	tableStr := m.renderTable(headerStyle, cellStyle)
	logContent := strings.Join(logger.GetLastLines(LogLines), "\n")
	infoText := fmt.Sprintf(
		"Press 'q' or Ctrl+C to quit (Last Updated: %s, Pending Updates: %d)",
		m.LastUpdate.Format("15:04:05"),
		len(m.BatchedUpdates),
	)

	performanceInfo := fmt.Sprintf(
		"Avg Update Time: %.2fms, CPU Usage: %.2f%%, Memory Usage: %d MB, Circular Buffer Size: %d, Goroutines: %d",
		m.getAverageUpdateTime().Seconds()*1000,
		m.CPUUsage,
		m.MemoryUsage/1024/1024,
		m.UpdateTimesSize,
		atomic.LoadInt64(&m.goroutineCount),
	)

	legendInfo := "Legend: ⬛️ = Waiting for other VMs in region, ⌛️/↻ = In Process, ✅/✓ = Success, ❌ = Failure"

	return lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		textBoxStyle.Render(logContent),
		infoStyle.Render(infoText),
		infoStyle.Render(performanceInfo),
		infoStyle.Render(legendInfo),
	)
}

// renderTable renders the main table of the display
func (m *DisplayModel) renderTable(headerStyle, cellStyle lipgloss.Style) string {
	var tableStr string
	tableStr += m.renderRow(DisplayColumns, headerStyle, true)
	if m.DebugMode {
		tableStr += strings.Repeat("-", AggregateColumnWidths()) + "\n"
	}

	machineSlice := m.getSortedMachineSlice()

	for _, machineInfo := range machineSlice {
		tableStr += m.renderRow(
			m.getMachineRowData(m.Deployment.Machines[machineInfo.Name]),
			cellStyle,
			false,
		)
	}
	return tableStr
}

// renderRow renders a single row of the table
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
		style := baseStyle.
			Width(DisplayColumns[i].Width).
			MaxWidth(DisplayColumns[i].Width)

		if DisplayColumns[i].EmojiColumn {
			if isHeader {
				style = style.Align(lipgloss.Center)
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
	return rowStr + "\n"
}

// getSortedMachineSlice returns a sorted slice of machine information
func (m *DisplayModel) getSortedMachineSlice() []struct {
	Location string
	Name     string
} {
	machineSlice := []struct {
		Location string
		Name     string
	}{}
	for _, machine := range m.Deployment.GetMachines() {
		if machine.GetName() != "" {
			machineSlice = append(machineSlice, struct {
				Location string
				Name     string
			}{
				Location: machine.GetDisplayLocation(),
				Name:     machine.GetName(),
			})
		}
	}
	slices.SortFunc(machineSlice, func(a, b struct {
		Location string
		Name     string
	}) int {
		if a.Location != b.Location {
			return strings.Compare(a.Location, b.Location)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return machineSlice
}

// getMachineRowData prepares the data for a machine row
func (m *DisplayModel) getMachineRowData(machine models.Machiner) []string {
	if machine.GetStartTime().IsZero() {
		machine.SetStartTime(time.Now())
	}
	elapsedTime := time.Since(machine.GetStartTime()).Truncate(TickerInterval)
	progress, total := machine.ResourcesComplete()
	progressBar := renderProgressBar(
		progress,
		total,
		DisplayColumns[4].Width-ProgressBarPadding,
	)

	return []string{
		machine.GetName(),
		machine.GetType().ShortResourceName,
		utils.TruncateString(machine.GetDisplayLocation(), DisplayColumns[2].Width-2),
		machine.GetStatusMessage(),
		progressBar,
		formatElapsedTime(elapsedTime),
		machine.GetPublicIP(),
		machine.GetPrivateIP(),
		ConvertOrchestratorToEmoji(machine.IsOrchestrator()),
		ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeSSH.Name)),
		ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeDocker.Name)),
		ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeBacalhau.Name)),
		ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeScript.Name)),
		"",
	}
}

// renderStyleByColumn applies styles based on the column content
func renderStyleByColumn(status string, style lipgloss.Style) lipgloss.Style {
	style = style.Bold(true).Align(lipgloss.Center)
	switch status {
	case models.DisplayTextSuccess:
		style = style.Foreground(lipgloss.Color("#00c413"))
	case models.DisplayTextWaiting:
		style = style.Foreground(lipgloss.Color("#69acdb"))
	case models.DisplayTextNotStarted:
		style = style.Foreground(lipgloss.Color("#2e2d2d"))
	case models.DisplayTextFailed:
		style = style.Foreground(lipgloss.Color("#ff0000"))
	}
	return style
}

// renderProgressBar creates a visual progress bar
func renderProgressBar(progress, total, width int) string {
	if total == 0 {
		return ""
	}
	filledWidth := int(math.Ceil(float64(progress) * float64(width) / float64(total)))
	emptyWidth := width - filledWidth
	if emptyWidth < 0 {
		emptyWidth = 0
	}

	filled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Render(strings.Repeat("█", filledWidth))
	empty := lipgloss.NewStyle().
		Foreground(lipgloss.Color("237")).
		Render(strings.Repeat("█", emptyWidth))

	return filled + empty
}

// formatElapsedTime formats a duration into a string
func formatElapsedTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % secondsPerMinute
	tenths := int(d.Milliseconds()/MillisecondsPerTenth) % TenthsPerSecond

	if minutes > 0 {
		return fmt.Sprintf("%dm%02d.%ds", minutes, seconds, tenths)
	}
	return fmt.Sprintf("%2d.%ds", seconds, tenths)
}

// AggregateColumnWidths calculates the total width of all columns
func AggregateColumnWidths() int {
	width := 0
	for _, column := range DisplayColumns {
		width += column.Width
	}
	return width
}

// ConvertOrchestratorToEmoji converts orchestrator status to an emoji
func ConvertOrchestratorToEmoji(orchestrator bool) string {
	if orchestrator {
		return models.DisplayTextOrchestratorNode
	}
	return models.DisplayTextWorkerNode
}

// ConvertStateToEmoji converts a service state to an emoji
func ConvertStateToEmoji(serviceState models.ServiceState) string {
	switch serviceState {
	case models.ServiceStateNotStarted:
		return models.DisplayTextNotStarted
	case models.ServiceStateSucceeded:
		return models.DisplayTextSuccess
	case models.ServiceStateUpdating:
		return models.DisplayTextWaiting
	case models.ServiceStateCreated:
		return models.DisplayTextCreating
	case models.ServiceStateFailed:
		return models.DisplayTextFailed
	}
	return models.DisplayTextWaiting
}

// getAverageUpdateTime calculates the average update time
func (m *DisplayModel) getAverageUpdateTime() time.Duration {
	var sum int64
	var count int
	for _, d := range m.UpdateTimes {
		if d != 0 {
			sum += d.Nanoseconds()
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return time.Duration(sum / int64(count))
}

// RenderFinalTable renders the final table with additional summary information
func (m *DisplayModel) RenderFinalTable() string {
	regularView := m.View()
	summaryInfo := m.generateSummaryInfo()
	return lipgloss.JoinVertical(
		lipgloss.Left,
		regularView,
		summaryInfo,
	)
}

func (m *DisplayModel) generateSummaryInfo() string {
	// Generate and format any final summary information
	// For example: total run time, number of successful/failed operations, etc.
	return fmt.Sprintf("Total Run Time: %s, Successful Operations: %d, Failed Operations: %d",
		m.getTotalRunTime(),
		m.getSuccessfulOperationsCount(),
		m.getFailedOperationsCount(),
	)
}
