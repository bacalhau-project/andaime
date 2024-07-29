package display

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	currentDisplay *Display
)

// GetCurrentDisplay returns the current display instance
func GetCurrentDisplay() *Display {
	return currentDisplay
}

const NumberOfCyclesToHighlight = 8
const HighlightTimer = 250 * time.Millisecond
const HighlightColor = tcell.ColorDarkGreen
const TextColor = tcell.ColorWhite
const timeoutDuration = 5
const textColumnPadding = 2
const MaxLogLines = 8

const RelativeSizeForTable = 5
const RelativeSizeForLogBox = 1

func NewDisplay(totalTasks int) *Display {
	d := newDisplayInternal(totalTasks, false)
	d.LogFileName = logger.GlobalLogPath
	currentDisplay = d
	return d
}

func newDisplayInternal(totalTasks int, testMode bool) *Display {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Display{
		Statuses:           make(map[string]*models.Status),
		App:                tview.NewApplication(),
		Table:              tview.NewTable().SetBorders(true),
		TotalTasks:         totalTasks,
		BaseHighlightColor: HighlightColor,
		FadeSteps:          NumberOfCyclesToHighlight,
		Quit:               utils.CreateStructChannel(1),
		TestMode:           testMode,
		LogBox:             tview.NewTextView().SetDynamicColors(true),
		LogFileName:        logger.GlobalLogPath,
		Ctx:                ctx,
		Cancel:             cancel,
		LogBuffer:          utils.NewCircularBuffer(MaxLogLines),
	}

	d.DebugLog = *logger.Get()
	d.Logger = *logger.Get()
	d.LogBox.SetBorder(true)
	d.LogBox.SetBorderColor(tcell.ColorWhite)
	d.LogBox.SetTitle("Log")
	d.LogBox.SetTitleColor(tcell.ColorWhite)
	d.setupLayout()

	return d
}

type DisplayColumn struct {
	Text         string
	Width        int
	Color        tcell.Color
	Align        int
	PaddingLeft  int
	PaddingRight int
	DataFunc     func(status models.Status) string
}

//nolint:gomnd
var DisplayColumns = []DisplayColumn{
	{
		Text:     "ID",
		Width:    10,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.ID },
	},
	{
		Text:     "Type",
		Width:    20,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.Type },
	},
	{
		Text:     "Location",
		Width:    15,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.Location },
	},
	{
		Text:     "Status",
		Width:    30,
		Color:    TextColor,
		DataFunc: func(status models.Status) string { return status.Status }},
	{Text: "Elapsed",
		Width: 10,
		Color: TextColor,
		Align: tview.AlignCenter,
		DataFunc: func(status models.Status) string {
			elapsedTime := time.Since(status.StartTime)
			//l := logger.Get()
			//l.Debugf("ID: %s, Elapsed time: %s", status.ID, elapsedTime)

			// Format the elapsed time
			minutes := int(elapsedTime.Minutes())
			seconds := float64(elapsedTime.Milliseconds()) / 1000.0

			if seconds < 10 {
				return fmt.Sprintf("%1.1fs", seconds)
			}
			if minutes == 0 {
				return fmt.Sprintf("%01.1fs", seconds)
			}
			return fmt.Sprintf("%dm%01.1fs", minutes, seconds)
		},
	},
	{
		Text:     "Instance ID",
		Width:    20,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.InstanceID }},
	{Text: "Public IP",
		Width:    15,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.PublicIP }},
	{Text: "Private IP",
		Width:    15,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.PrivateIP }},
}

func (d *Display) setupLayout() {
	d.DebugLog.Debug("Setting up layout")
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.Table, 0, 3, true).
		AddItem(d.LogBox, 12, 1, false)

	d.App.SetRoot(flex, true).EnableMouse(false)
}

func (d *Display) UpdateStatus(newStatus *models.Status) {
	if d == nil || newStatus == nil {
		d.Logger.Infof("Invalid state in UpdateStatus: d=%v, status=%v", d, newStatus)
		return
	}

	// d.Logger.Debugf("UpdateStatus called ID: %s", newStatus.ID)

	d.StatusesMu.Lock()

	if d.Statuses == nil {
		d.Statuses = make(map[string]*models.Status)
	}

	if _, exists := d.Statuses[newStatus.ID]; !exists {
		// d.Logger.Debugf("Adding new status ID: %s", newStatus.ID)
		d.Statuses[newStatus.ID] = newStatus
	} else {
		// d.Logger.Debugf("Updating existing status ID: %s", newStatus.ID)
		s := d.Statuses[newStatus.ID]
		d.Statuses[newStatus.ID] = utils.UpdateStatus(s, newStatus)
	}
	d.StatusesMu.Unlock()

	if newStatus.Type != "VM" {
		d.displayResourceProgress(newStatus)
	}

	d.scheduleUpdate()
}

func (d *Display) getHighlightColor(cycles int) tcell.Color {
	if cycles <= 0 {
		return tcell.ColorDefault
	}

	// Convert HighlightColor to RGB
	baseR, baseG, baseB := HighlightColor.RGB()

	// End with white
	targetR, targetG, targetB := tcell.ColorWhite.RGB()

	stepR := float64(targetR-baseR) / float64(d.FadeSteps)
	stepG := float64(targetG-baseG) / float64(d.FadeSteps)
	stepB := float64(targetB-baseB) / float64(d.FadeSteps)

	fadeProgress := d.FadeSteps - cycles
	r := uint8(float64(baseR) + stepR*float64(fadeProgress))
	g := uint8(float64(baseG) + stepG*float64(fadeProgress))
	b := uint8(float64(baseB) + stepB*float64(fadeProgress))

	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}

func (d *Display) getTableString() string {
	var tableContent strings.Builder
	tableContent.WriteString(d.getTableHeader())
	if len(d.LastTableState) > 1 {
		for _, row := range d.LastTableState[1:] {
			tableContent.WriteString(d.getTableRow(row))
		}
	}
	tableContent.WriteString(d.getTableFooter())
	return tableContent.String()
}

func (d *Display) getTableRow(row []string) string {
	var rowContent strings.Builder
	rowContent.WriteString("│")
	for col, cell := range row {
		paddedText := d.padText(cell, DisplayColumns[col].Width)
		rowContent.WriteString(fmt.Sprintf(" %s │", paddedText))
	}
	rowContent.WriteString("\n")
	return rowContent.String()
}

func (d *Display) getTableHeader() string {
	var header strings.Builder
	header.WriteString("┌")
	for i, column := range DisplayColumns {
		header.WriteString(strings.Repeat("─", column.Width+textColumnPadding))
		if i < len(DisplayColumns)-1 {
			header.WriteString("┬")
		}
	}
	header.WriteString("┐\n")

	header.WriteString("│")
	for _, column := range DisplayColumns {
		header.WriteString(fmt.Sprintf(" %-*s │", column.Width, column.Text))
	}
	header.WriteString("\n")

	header.WriteString("├")
	for i, column := range DisplayColumns {
		header.WriteString(strings.Repeat("─", column.Width+textColumnPadding))
		if i < len(DisplayColumns)-1 {
			header.WriteString("┼")
		}
	}
	header.WriteString("┤\n")

	return header.String()
}

func (d *Display) getTableFooter() string {
	var footer strings.Builder
	footer.WriteString("└")
	for i, column := range DisplayColumns {
		footer.WriteString(strings.Repeat("─", column.Width+textColumnPadding))
		if i < len(DisplayColumns)-1 {
			footer.WriteString("┴")
		}
	}
	footer.WriteString("┘\n")
	return footer.String()
}

func (d *Display) padText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}

func (d *Display) Start(sigChan chan os.Signal) {
	if d.Logger.Logger == nil {
		d.Logger = *logger.Get()
	}
	d.Logger.Debug("Starting display")
	d.Statuses = make(map[string]*models.Status)

	stopChan := utils.CreateSignalChannel(1)
	allDone := utils.CreateStructChannel(1)

	go func() {
		d.Logger.Debug("Starting signal handler goroutine")
		select {
		case <-sigChan:
			d.Logger.Debug("Received signal, stopping display")
			close(stopChan)
		case <-d.Quit:
			d.Logger.Debug("Stop channel closed, stopping display")
			close(stopChan)
		}
	}()

	if !d.TestMode {
		go func() {
			d.Logger.Debug("Setting up table")
			d.Table.SetTitle("Deployment Status")
			d.Table.SetBorder(true)
			d.Table.SetBorderColor(tcell.ColorWhite)
			d.Table.SetTitleColor(tcell.ColorWhite)

			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			d.Logger.Debug("Starting update loop")
			for {
				select {
				case <-stopChan:
					d.Logger.Debug("Received stop signal in update loop")
					d.Stop()
					return
				case <-ticker.C:
					d.updateFromGlobalMap()
					d.renderTable()
					d.updateLogBox()
				}
			}
		}()

		d.Logger.Debug("Starting tview application")
		go func() {
			if err := d.App.Run(); err != nil {
				d.Logger.Errorf("Error running display: %v", err)
			}
			d.Logger.Debug("tview application stopped")
			utils.CloseChannel(stopChan)
		}()

		go func() {
			<-stopChan
			d.Logger.Debug("Stop channel closed, waiting for goroutines to finish")
			utils.CloseChannel(allDone)
		}()

		select {
		case <-allDone:
			d.Logger.Debug("All goroutines finished")
		case <-time.After(10 * time.Second):
			d.Logger.Warn("Timeout waiting for goroutines to finish")
			d.Logger.Debug("Channels still open:")
			if !utils.IsChannelClosed(d.Quit) {
				d.Logger.Debug("- Quit channel is still open")
			}
		}
	} else {
		d.Logger.Debug("Running in test mode")
		// In test mode, just render to the virtual console
		d.renderToVirtualConsole()
	}
	d.Logger.Debug("Display Start method completed")
	d.Stop() // Ensure the display stops after completion
}

func (d *Display) updateFromGlobalMap() {
	StatusMutex.RLock()
	defer StatusMutex.RUnlock()

	for id, status := range GlobalStatusMap {
		d.Statuses[id] = status
	}
}

func (d *Display) Stop() {
	d.Logger.Debug("Stopping display")
	d.App.QueueUpdateDraw(func() {
		d.App.Stop()
	})
	d.resetTerminal()
	utils.CloseChannel(d.Quit)
}

func (d *Display) resetTerminal() {
	l := logger.Get()
	l.Debug("Resetting terminal")
	if !d.TestMode {
		d.App.Suspend(func() {
			fmt.Print("\033[?1049l") // Exit alternate screen buffer
			fmt.Print("\033[?25h")   // Show cursor
		})
	}
}

func (d *Display) WaitForStop() {
	d.Logger.Debug("Waiting for display to stop")
	select {
	case <-d.Quit:
		d.Logger.Debug("Display stopped")
	case <-time.After(5 * time.Second): //nolint:gomnd
		d.Logger.Debug("Timeout waiting for display to stop")
	}
}

func (d *Display) AddLogEntry(logEntry string) {
	d.DebugLog.Debug(logEntry)
	if d.TestMode {
		_, err := d.LogBox.Write([]byte(logEntry + "\n"))
		if err != nil {
			d.DebugLog.Error(fmt.Sprintf("Error writing to log box: %v", err))
		}
	} else {
		d.App.QueueUpdateDraw(func() {
			fmt.Fprintf(d.LogBox, "%s\n", logEntry)
		})
	}
}

func (d *Display) printFinalTableState() {
	d.DebugLog.Debug("Printing final table state")
	if len(d.LastTableState) == 0 {
		fmt.Println("No data to display")
		return
	}

	// Determine column widths
	colWidths := make([]int, len(d.LastTableState[0]))
	for _, row := range d.LastTableState {
		for col, cell := range row {
			if len(cell) > colWidths[col] {
				colWidths[col] = len(cell)
			}
		}
	}

	// Function to print a row separator
	printSeparator := func() {
		for col, width := range colWidths {
			if col == 0 {
				fmt.Print("+")
			}
			fmt.Print(strings.Repeat("-", width+2)) //nolint:gomnd
			fmt.Print("+")
		}
		fmt.Println()
	}

	// Clear screen and move cursor to top-left
	fmt.Print("\033[2J\033[H")

	printSeparator() // Print top separator

	// Print header row
	for col, cell := range d.LastTableState[0] {
		fmt.Printf("| %-*s ", colWidths[col], cell)
	}
	fmt.Println("|")
	printSeparator() // Print separator after header

	// Print the table
	for _, row := range d.LastTableState[1:] {
		for col, cell := range row {
			fmt.Printf("| %-*s ", colWidths[col], cell)
		}
		fmt.Println("|")
	}
	printSeparator() // Print bottom separator
}
func (d *Display) scheduleUpdate() {
	d.UpdateMutex.Lock()
	defer d.UpdateMutex.Unlock()

	// l := logger.Get()
	// l.Debug("Scheduling update")

	if !d.UpdatePending {
		d.UpdatePending = true
		// l.Debug("Update pending")
		go func() {
			time.Sleep(250 * time.Millisecond) // Increased delay to reduce update frequency
			d.App.QueueUpdateDraw(func() {
				d.UpdateMutex.Lock()
				d.UpdatePending = false
				d.UpdateMutex.Unlock()
				d.updateDisplay()
			})
		}()
	}
}

func (d *Display) updateDisplay() {
	d.renderTable()
	d.updateLogBox()
}

func (d *Display) renderTable() {
	// Add header row
	for col, column := range DisplayColumns {
		cell := tview.NewTableCell(column.Text).
			SetMaxWidth(column.Width).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter)
		d.Table.SetCell(0, col, cell)
	}

	nodes := make([]*models.Status, 0)
	for _, status := range d.Statuses {
		if status.Type == "VM" {
			nodes = append(nodes, status)
		} else {
			d.displayResourceProgress(status)
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// Initialize lastTableState with header row
	d.LastTableState = [][]string{make([]string, len(DisplayColumns))}
	for col, column := range DisplayColumns {
		d.LastTableState[0][col] = column.Text
	}

	for row, node := range nodes {
		tableRow := make([]string, len(DisplayColumns))
		for col, column := range DisplayColumns {
			cellText := column.DataFunc(*node)
			tableRow[col] = cellText
			cell := tview.NewTableCell(cellText).
				SetMaxWidth(column.Width).
				SetTextColor(column.Color).
				SetAlign(column.Align)
			d.Table.SetCell(row+1, col, cell)
		}
		d.LastTableState = append(d.LastTableState, tableRow)
	}

}

func (d *Display) displayResourceProgress(status *models.Status) {
	progressText := fmt.Sprintf("%s: %s - %s", status.Type, status.ID, status.Status)
	_, _ = d.LogBox.Write([]byte(progressText + "\n"))
}

func (d *Display) updateLogBox() {
	lines := logger.GetLastLines(d.LogFileName, MaxLogLines)
	d.LogBox.Clear()
	for _, line := range lines {
		fmt.Fprintln(d.LogBox, line)
	}
}

func (d *Display) Log(message string) {
	d.Logger.Info(message)
	d.scheduleUpdate()
}

func (d *Display) renderToVirtualConsole() {
	if d.VirtualConsole == nil {
		d.VirtualConsole = &bytes.Buffer{}
	}

	d.VirtualConsole.Reset()
	d.VirtualConsole.WriteString(d.getTableString())
	d.VirtualConsole.WriteString("\nLog:\n")
	if d.LogBox != nil {
		d.VirtualConsole.WriteString(d.LogBox.GetText(false))
	}
}
