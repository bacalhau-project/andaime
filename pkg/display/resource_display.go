package display

import (
	"context"
	"fmt"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const NumberOfCyclesToHighlight = 8
const HighlightTimer = 250 * time.Millisecond
const HighlightColor = tcell.ColorDarkGreen
const TextColor = tcell.ColorWhite
const timeoutDuration = 5
const textColumnPadding = 2
const MaxLogLines = 8

const RelativeSizeForTable = 5
const RelativeSizeForLogBox = 1

func NewDisplay() *Display {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Display{
		Statuses:   make(map[string]*models.Status),
		App:        tview.NewApplication(),
		Table:      tview.NewTable().SetBorders(true),
		LogBox:     tview.NewTextView().SetDynamicColors(true),
		Ctx:        ctx,
		Cancel:     cancel,
		Logger:     logger.Get(),
		updateChan: utils.CreateStructChannel("display_update_chan", 1),
	}

	d.LogBox.SetBorder(true).
		SetBorderColor(tcell.ColorWhite).
		SetTitle("Log").
		SetTitleColor(tcell.ColorWhite)
	d.setupLayout()

	d.Logger.Debugf("Display created with updateChan: %v", d.updateChan)

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
		Width:    8,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.Type },
	},
	{
		Text:     "Location",
		Width:    12,
		Color:    TextColor,
		Align:    tview.AlignCenter,
		DataFunc: func(status models.Status) string { return status.Location },
	},
	{
		Text:     "Status",
		Width:    36,
		Color:    TextColor,
		DataFunc: func(status models.Status) string { return status.Status }},
	{
		Text:  "Elapsed",
		Width: 10,
		Color: TextColor,
		Align: tview.AlignCenter,
		DataFunc: func(status models.Status) string {
			elapsedTime := time.Since(status.StartTime)
			//l := logger.Get()
			//l.Debugf("ID: %s, Elapsed time: %s", status.ID, elapsedTime)

			// Format the elapsed time
			minutes := int(elapsedTime.Minutes())
			seconds := float64(elapsedTime.Milliseconds()%60000) / 1000.0

			if minutes < 1 && seconds < 10 {
				return fmt.Sprintf("%1.1fs", seconds)
			} else if minutes < 1 {
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
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.Table, 0, 3, true).
		AddItem(d.LogBox, 12, 1, false)

	d.App.SetRoot(flex, true).EnableMouse(false)
}

func (d *Display) UpdateStatus(newStatus *models.Status) {
	l := logger.Get()

	if d == nil || newStatus == nil {
		l.Infof("Invalid state in UpdateStatus: d=%v, status=%v", d, newStatus)
		return
	}

	d.StatusesMu.Lock()
	if d.Statuses == nil {
		d.Statuses = make(map[string]*models.Status)
	}

	if _, exists := d.Statuses[newStatus.ID]; !exists {
		//d.Logger.Debugf("New status added: %s", newStatus.ID)
		d.Statuses[newStatus.ID] = newStatus
	} else {
		// d.Logger.Debugf("Status updated: %s", newStatus.ID)
		d.Statuses[newStatus.ID] = utils.UpdateStatus(d.Statuses[newStatus.ID], newStatus)
	}
	d.StatusesMu.Unlock()

	d.displayResourceProgress(newStatus)

	select {
	case d.updateChan <- struct{}{}:
		//	d.Logger.Debugf("Update signal sent for status: %s", newStatus.ID)
		_ = d.updateChan
	default:
		//	d.Logger.Debugf("Update channel full, skipped update for status: %s", newStatus.ID)
		_ = d.updateChan
	}
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

func (d *Display) GetTableString() string {
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
	padding := width - len(text)
	leftPadding := padding / 2 //nolint:gomnd
	rightPadding := padding - leftPadding
	return strings.Repeat(" ", leftPadding) + text + strings.Repeat(" ", rightPadding)
}

func (d *Display) Start() {
	d.Logger.Debug("Starting display")
	d.StopChan = utils.CreateStructChannel("display_stop_chan", 1)

	d.Table.SetTitle("Deployment Status").
		SetBorder(true).
		SetBorderColor(tcell.ColorWhite).
		SetTitleColor(tcell.ColorWhite)

	go func() {
		d.Logger.Debug("Display goroutine started")
		defer func() {
			if r := recover(); r != nil {
				d.Logger.Errorf("Panic in display goroutine: %v", r)
				d.Logger.Debugf("Stack trace:\n%s", debug.Stack())
			}
			d.Logger.Debug("Display goroutine exiting")
		}()

		updateTimeout := time.NewTimer(30 * time.Second)
		defer updateTimeout.Stop()

		for {
			select {
			case <-d.Ctx.Done():
				// d.Logger.Debug("Context cancelled, stopping display")
				return
			case <-d.StopChan:
				// d.Logger.Debug("Stop signal received, stopping display")
				return
			case <-d.updateChan:
				// d.Logger.Debug("Update signal received")
				updateTimeout.Reset(30 * time.Second)
				d.App.QueueUpdateDraw(func() {
					d.renderTable()
					d.updateLogBox()
				})
			case <-updateTimeout.C:
				d.Logger.Warn("Update timeout reached, possible hang detected")
				// Optionally, you could trigger some recovery action here
			}
		}
	}()

	d.Logger.Debug("Starting tview application")
	d.DisplayRunning = true
	if err := d.App.Run(); err != nil {
		d.DisplayRunning = false
		d.Logger.Errorf("Error running display: %v", err)
	}
	d.Logger.Debug("tview application stopped")
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
	d.Cancel() // Cancel the context

	d.Logger.Debug("Closing StopChan")

	// Stop the application in a separate goroutine to avoid deadlock
	stopDone := utils.CreateBoolChannel("stop_done", 1)
	go func() {
		defer utils.CloseChannel(stopDone)
		d.Logger.Debug("Stopping tview application")
		if d.DisplayRunning {
			d.App.Stop()
		}
		d.Logger.Debug("tview application stopped")
	}()

	// Wait for the application to stop with a timeout
	select {
	case <-stopDone:
		d.Logger.Debug("Application stop confirmed")
	case <-time.After(5 * time.Second):
		d.Logger.Warn("Timeout waiting for application to stop")
	}

	d.Logger.Debug("Closing updateChan")
	utils.CloseChannel(d.updateChan)

	d.Logger.Debug("Closing all registered channels")
	utils.CloseAllChannels()

	defer d.DumpGoroutines()
	d.resetTerminal()
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

func (d *Display) resetTerminal() {
	l := logger.Get()
	l.Debug("Resetting terminal")
	d.App.Suspend(func() {
		fmt.Print("\033[?1049l") // Exit alternate screen buffer
		fmt.Print("\033[?25h")   // Show cursor
	})
	fmt.Print(logger.GlobalLoggedBuffer.String())
}

func (d *Display) AddLogEntry(logEntry string) {
	d.App.QueueUpdateDraw(func() {
		fmt.Fprintf(d.LogBox, "%s\n", logEntry)
	})
}

func (d *Display) printFinalTableState() {
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
	d.UpdateMu.Lock()
	defer d.UpdateMu.Unlock()

	if !d.UpdatePending {
		d.UpdatePending = true
		d.Logger.Debug("Update scheduled")
		go func() {
			time.Sleep(250 * time.Millisecond) // Increased delay to reduce update frequency
			d.App.QueueUpdateDraw(func() {
				d.UpdateMu.Lock()
				d.UpdatePending = false
				d.UpdateMu.Unlock()
				d.updateDisplay()
			})
		}()
	} else {
		d.Logger.Debug("Update already pending, skipping")
	}
}

func (d *Display) updateDisplay() {
	d.renderTable()
	d.updateLogBox()
}

func (d *Display) renderTable() {
	// Add header row
	for col, column := range DisplayColumns {
		textWithPadding := d.padText(column.Text, column.Width)
		cell := tview.NewTableCell(textWithPadding).
			SetMaxWidth(column.Width).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter)
		d.Table.SetCell(0, col, cell)
	}

	resources := make([]*models.Status, 0)
	for _, status := range d.Statuses {
		resources = append(resources, status)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].ID < resources[j].ID
	})

	// Initialize lastTableState with header row
	d.LastTableState = [][]string{make([]string, len(DisplayColumns))}
	for col, column := range DisplayColumns {
		d.LastTableState[0][col] = column.Text
	}

	for row, resource := range resources {
		tableRow := make([]string, len(DisplayColumns))
		for col, column := range DisplayColumns {
			cellText := column.DataFunc(*resource)
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
	lines := logger.GetLastLines(logger.GlobalLogPath, MaxLogLines)
	d.LogBox.Clear()
	for _, line := range lines {
		fmt.Fprintln(d.LogBox, line)
	}
}

func (d *Display) Log(message string) {
	d.Logger.Info(message)
	d.scheduleUpdate()
}

func (d *Display) DumpGoroutines() {
	_, _ = fmt.Fprintf(&logger.GlobalLoggedBuffer, "pprof at end of executeCreateDeployment\n")
	_ = pprof.Lookup("goroutine").WriteTo(&logger.GlobalLoggedBuffer, 1)
}
