package display

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var debugLogger *log.Logger

func init() {
	debugFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) //nolint:gomnd
	if err != nil {
		log.Fatal(err)
	}
	debugLogger = log.New(debugFile, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func logDebugf(format string, v ...interface{}) {
	debugLogger.Printf(format, v...)
}

const NumberOfCyclesToHighlight = 8
const HighlightTimer = 250 * time.Millisecond
const HighlightColor = tcell.ColorDarkGreen
const timeoutDuration = 5
const textColumnPadding = 2

const RelativeSizeForTable = 5
const RelativeSizeForLogBox = 1

type Status struct {
	ID              string
	Type            string
	Region          string
	Zone            string
	Status          string
	DetailedStatus  string
	ElapsedTime     time.Duration
	InstanceID      string
	PublicIP        string
	PrivateIP       string
	HighlightCycles int
}

type Display struct {
	statuses           map[string]*Status
	statusesMu         sync.RWMutex
	app                *tview.Application
	table              *tview.Table
	totalTasks         int
	completedTasks     int
	baseHighlightColor tcell.Color
	fadeSteps          int
	stopOnce           sync.Once
	stopChan           chan struct{}
	quit               chan struct{}
	lastTableState     [][]string
	DebugLog           logger.Logger
	Logger             logger.Logger
	LogBox             *tview.TextView
	testMode           bool
}

type DisplayColumn struct {
	Text     string
	Width    int
	Color    tcell.Color
	DataFunc func(status Status) string
}

//nolint:gomnd
var DisplayColumns = []DisplayColumn{
	{Text: "ID", Width: 10, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.ID }},
	{Text: "Type", Width: 10, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.Type }},
	{Text: "Region", Width: 15, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.Region }},
	{Text: "Zone", Width: 15, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.Zone }},
	{Text: "Status",
		Width:    30,
		Color:    tcell.ColorRed,
		DataFunc: func(status Status) string { return fmt.Sprintf("%s (%s)", status.Status, status.DetailedStatus) }},
	{Text: "Elapsed",
		Width:    10,
		Color:    tcell.ColorRed,
		DataFunc: func(status Status) string { return status.ElapsedTime.Round(time.Second).String() }},
	{Text: "Instance ID",
		Width:    20,
		Color:    tcell.ColorRed,
		DataFunc: func(status Status) string { return status.InstanceID }},
	{Text: "Public IP",
		Width:    15,
		Color:    tcell.ColorRed,
		DataFunc: func(status Status) string { return status.PublicIP }},
	{Text: "Private IP",
		Width:    15,
		Color:    tcell.ColorRed,
		DataFunc: func(status Status) string { return status.PrivateIP }},
}

func NewDisplay(totalTasks int) *Display {
	return newDisplayInternal(totalTasks, false)
}

func newDisplayInternal(totalTasks int, testMode bool) *Display {
	d := &Display{
		statuses:           make(map[string]*Status),
		app:                tview.NewApplication(),
		table:              tview.NewTable().SetBorders(true),
		totalTasks:         totalTasks,
		baseHighlightColor: HighlightColor,
		fadeSteps:          NumberOfCyclesToHighlight,
		stopChan:           make(chan struct{}),
		quit:               make(chan struct{}),
		testMode:           testMode,
		LogBox:             tview.NewTextView().SetDynamicColors(true),
	}

	d.DebugLog = *logger.Get()
	d.Logger = *logger.Get()
	d.setupTable()
	d.setupLayout()

	return d
}

func (d *Display) setupTable() {
	d.DebugLog.Debug("Setting up table")
	d.table.SetFixed(1, len(DisplayColumns))
	d.table.SetBordersColor(tcell.ColorWhite)
	d.table.SetBorders(true)

	for col, header := range DisplayColumns {
		cell := tview.NewTableCell(header.Text).
			SetTextColor(header.Color).
			SetSelectable(false).
			SetExpansion(0).
			SetMaxWidth(header.Width)
		d.table.SetCell(0, col, cell)
	}
	d.table.SetBorderPadding(0, 0, 1, 1)
}

func (d *Display) setupLayout() {
	d.DebugLog.Debug("Setting up layout")
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.table, 0, 3, true). //nolint:gomnd
		AddItem(d.LogBox, 5, 1, false)

	d.app.SetRoot(flex, true).EnableMouse(false)
}

func (d *Display) UpdateStatus(status *Status) chan struct{} {
	d.DebugLog.Debugf("UpdateStatus called ID: %s", status.ID)
	updateComplete := make(chan struct{})

	d.statusesMu.RLock()
	existingStatus, exists := d.statuses[status.ID]
	d.statusesMu.RUnlock()

	newStatus := *status // Create a copy of the status
	if !exists {
		d.DebugLog.Debugf("Adding new status ID: %s", status.ID)
		d.statusesMu.Lock()
		d.completedTasks++
		newStatus.HighlightCycles = d.fadeSteps
		d.statuses[newStatus.ID] = &newStatus
		d.statusesMu.Unlock()
	} else if *existingStatus != newStatus {
		d.DebugLog.Debugf("Updating existing status ID: %s", status.ID)
		d.statusesMu.Lock()
		newStatus.HighlightCycles = d.fadeSteps
		d.statuses[newStatus.ID] = &newStatus
		d.statusesMu.Unlock()
	}

	d.DebugLog.Debugf("Queueing table render for status ID: %s", status.ID)
	d.app.QueueUpdateDraw(func() {
		d.DebugLog.Debugf("Starting table render for status ID: %s", status.ID)
		d.renderTable()
		d.DebugLog.Debugf("Finished table render for status ID: %s", status.ID)
		close(updateComplete)
	})
	d.DebugLog.Debugf("UpdateStatus queued for status ID: %s", status.ID)
	return updateComplete
}

func (d *Display) getHighlightColor(cycles int) tcell.Color {
	if cycles <= 0 {
		return tcell.ColorDefault
	}

	// Convert HighlightColor to RGB
	baseR, baseG, baseB := HighlightColor.RGB()

	// End with white
	targetR, targetG, targetB := tcell.ColorWhite.RGB()

	stepR := float64(targetR-baseR) / float64(d.fadeSteps)
	stepG := float64(targetG-baseG) / float64(d.fadeSteps)
	stepB := float64(targetB-baseB) / float64(d.fadeSteps)

	fadeProgress := d.fadeSteps - cycles
	r := uint8(float64(baseR) + stepR*float64(fadeProgress))
	g := uint8(float64(baseG) + stepG*float64(fadeProgress))
	b := uint8(float64(baseB) + stepB*float64(fadeProgress))

	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}

func (d *Display) startHighlightTimer() {
	if d.testMode {
		return // Don't start the timer in test mode
	}
	d.DebugLog.Debug("Starting highlight timer")
	go func() {
		ticker := time.NewTicker(HighlightTimer)
		defer ticker.Stop()
		tickCount := 0
		for {
			select {
			case <-ticker.C:
				tickCount++
				d.DebugLog.Debug(fmt.Sprintf("Highlight timer tick %d started", tickCount))

				updateComplete := make(chan bool)
				timeoutTimer := time.NewTimer(timeoutDuration * time.Second)

				go func() {
					d.app.QueueUpdateDraw(func() {
						d.DebugLog.Debug(fmt.Sprintf("QueueUpdateDraw in timer tick %d started", tickCount))
						d.renderTable()
						d.DebugLog.Debug(fmt.Sprintf("QueueUpdateDraw in timer tick %d completed", tickCount))
						updateComplete <- true
					})
				}()

				select {
				case <-updateComplete:
					d.DebugLog.Debug(fmt.Sprintf("Highlight timer tick %d completed successfully", tickCount))
				case <-timeoutTimer.C:
					d.DebugLog.Debug(fmt.Sprintf("Highlight timer tick %d timed out", tickCount))
				}

				timeoutTimer.Stop()

			case <-d.stopChan:
				d.DebugLog.Debug("Highlight timer stopped")
				return
			}
		}
	}()
}

func (d *Display) renderTable() {
	logDebugf("Rendering table started")
	d.statusesMu.RLock()
	statusesCopy := make(map[string]*Status, len(d.statuses))
	for id, status := range d.statuses {
		statusCopy := *status
		statusesCopy[id] = &statusCopy
	}
	d.statusesMu.RUnlock()

	logDebugf("Statuses copied: %d statuses", len(statusesCopy))

	statuses := make([]*Status, 0, len(statusesCopy))
	for _, status := range statusesCopy {
		statuses = append(statuses, status)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})

	logDebugf("Statuses sorted")

	d.lastTableState = make([][]string, len(statuses)+1)
	d.lastTableState[0] = make([]string, len(DisplayColumns))
	for col, header := range DisplayColumns {
		d.lastTableState[0][col] = header.Text
	}

	var tableContent strings.Builder
	tableContent.WriteString(d.getTableHeader())

	for row, status := range statuses {
		highlightColor := d.getHighlightColor(status.HighlightCycles)
		d.lastTableState[row+1] = make([]string, len(DisplayColumns))
		rowContent := d.getTableRow(status, highlightColor)
		tableContent.WriteString(rowContent)

		for col, column := range DisplayColumns {
			cellText := column.DataFunc(*status)
			paddedText := d.padText(cellText, column.Width)
			d.lastTableState[row+1][col] = cellText
			cell := tview.NewTableCell(paddedText).
				SetMaxWidth(column.Width).
				SetTextColor(highlightColor)
			d.table.SetCell(row+1, col, cell)
		}
	}

	tableContent.WriteString(d.getTableFooter())

	if d.testMode {
		logDebugf("Test mode: Adding log entry for table content")
		d.AddLogEntry(tableContent.String())
	}

	d.app.QueueUpdateDraw(func() {
		logDebugf("Table update queued")
	})

	logDebugf("Table cells set")

	d.statusesMu.Lock()
	for id, status := range d.statuses {
		if status.HighlightCycles > 0 {
			status.HighlightCycles--
			d.statuses[id] = status
		}
	}
	d.statusesMu.Unlock()
	logDebugf("Table rendered")
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

func (d *Display) getTableRow(status *Status, _ tcell.Color) string {
	var row strings.Builder
	row.WriteString("│")
	for _, column := range DisplayColumns {
		cellText := column.DataFunc(*status)
		paddedText := d.padText(cellText, column.Width)
		row.WriteString(fmt.Sprintf(" %s │", paddedText))
	}
	row.WriteString("\n")
	return row.String()
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
	logDebugf("Starting display")
	if d.testMode {
		logDebugf("Test mode: Starting highlight timer")
		d.startHighlightTimer()
		return
	}

	go func() {
		logDebugf("Setting up input capture")
		d.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			logDebugf("Key event received: %v", event.Key())
			if event.Key() == tcell.KeyCtrlC || event.Rune() == 'q' {
				logDebugf("Ctrl+C or 'q' detected, stopping display")
				d.Stop()
				return nil
			}
			return event
		})

		logDebugf("Starting highlight timer")
		d.startHighlightTimer()

		logDebugf("Running tview application")
		if err := d.app.Run(); err != nil {
			logDebugf("Error running display: %v", err)
		}
		logDebugf("tview application finished running")
	}()

	go func() {
		logDebugf("Starting signal handling goroutine")
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		select {
		case <-d.stopChan:
			logDebugf("Stop signal received from internal channel")
		case sig := <-signalChan:
			logDebugf("Stop signal received from external channel: %v", sig)
		}
		logDebugf("Stopping app")
		d.Stop()
	}()
	logDebugf("Display start completed")
}

func (d *Display) Stop() {
	d.DebugLog.Debug("Stopping display")
	d.stopOnce.Do(func() {
		close(d.stopChan)
		if !d.testMode {
			d.WaitForStop()
		} else {
			close(d.quit) // Close quit channel immediately in test mode
		}
		d.printFinalTableState()
		d.resetTerminal()
	})
}

func (d *Display) resetTerminal() {
	logDebugf("Resetting terminal")
	if !d.testMode {
		d.app.Suspend(func() {
			fmt.Print("\033[?1049l") // Exit alternate screen buffer
			fmt.Print("\033[?25h")   // Show cursor
		})
	}
}

func (d *Display) WaitForStop() {
	d.DebugLog.Debug("Waiting for display to stop")
	select {
	case <-d.quit:
		d.DebugLog.Debug("Display stopped")
	case <-time.After(5 * time.Second): //nolint:gomnd
		d.DebugLog.Debug("Timeout waiting for display to stop")
	}
}

func (d *Display) AddLogEntry(logEntry string) {
	d.DebugLog.Debug(logEntry)
	if d.testMode {
		_, err := d.LogBox.Write([]byte(logEntry + "\n"))
		if err != nil {
			d.DebugLog.Error(fmt.Sprintf("Error writing to log box: %v", err))
		}
	} else {
		d.app.QueueUpdateDraw(func() {
			fmt.Fprintf(d.LogBox, "%s\n", logEntry)
		})
	}
}

func (d *Display) printFinalTableState() {
	d.DebugLog.Debug("Printing final table state")
	if len(d.lastTableState) == 0 {
		fmt.Println("No data to display")
		return
	}

	// Determine column widths
	colWidths := make([]int, len(d.lastTableState[0]))
	for _, row := range d.lastTableState {
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
	for col, cell := range d.lastTableState[0] {
		fmt.Printf("| %-*s ", colWidths[col], cell)
	}
	fmt.Println("|")
	printSeparator() // Print separator after header

	// Print the table
	for _, row := range d.lastTableState[1:] {
		for col, cell := range row {
			fmt.Printf("| %-*s ", colWidths[col], cell)
		}
		fmt.Println("|")
	}
	printSeparator() // Print bottom separator
}
