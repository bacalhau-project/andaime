package display

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/utils"
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
const MaxLogLines = 8

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
	LogFileName        string
	logFile            *os.File
	LogBox             *tview.TextView
	testMode           bool
	ctx                context.Context
	cancel             context.CancelFunc
	logBuffer          *utils.CircularBuffer
	updatePending      bool
	updateMutex        sync.Mutex
}

func NewDisplay(totalTasks int) *Display {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Display{
		statuses:           make(map[string]*Status),
		app:                tview.NewApplication(),
		table:              tview.NewTable().SetBorders(true),
		totalTasks:         totalTasks,
		baseHighlightColor: HighlightColor,
		fadeSteps:          NumberOfCyclesToHighlight,
		stopChan:           make(chan struct{}),
		quit:               make(chan struct{}),
		testMode:           false,
		LogBox:             tview.NewTextView().SetDynamicColors(true),
		ctx:                ctx,
		cancel:             cancel,
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

func newDisplayInternal(totalTasks int, testMode bool) *Display {
	ctx, cancel := context.WithCancel(context.Background())
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
		ctx:                ctx,
		cancel:             cancel,
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
		AddItem(d.table, RelativeSizeForTable, 0, true). //nolint:gomnd
		AddItem(d.LogBox, RelativeSizeForLogBox, 0, false)

	d.app.SetRoot(flex, true).EnableMouse(false)
}

func (d *Display) UpdateStatus(status *Status) {
	if d == nil || status == nil {
		d.Logger.Infof("Invalid state in UpdateStatus: d=%v, status=%v", d, status)
		return
	}

	d.Logger.Infof("UpdateStatus called ID: %s", status.ID)

	d.statusesMu.Lock()
	defer d.statusesMu.Unlock()

	newStatus := *status // Create a copy of the status
	if _, exists := d.statuses[newStatus.ID]; !exists {
		d.completedTasks++
	}
	newStatus.HighlightCycles = d.fadeSteps
	d.statuses[newStatus.ID] = &newStatus

	d.app.QueueUpdateDraw(func() {
		d.renderTable()
		d.updateLogBox()
	})
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

func (d *Display) getTableString() string {
	var tableContent strings.Builder
	tableContent.WriteString(d.getTableHeader())
	if len(d.lastTableState) > 1 {
		for _, row := range d.lastTableState[1:] {
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
	d.stopChan = make(chan struct{})
	d.quit = make(chan struct{})
	d.statuses = make(map[string]*Status)
	go func() {
		<-d.stopChan
		close(d.quit)
	}()

	go func() {
		for {
			select {
			case <-d.stopChan:
				return
			default:
				d.app.QueueUpdateDraw(func() {
					d.renderTable()
					d.updateLogBox()
				})
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	if err := d.app.Run(); err != nil {
		d.Logger.Errorf("Error running display: %v", err)
	}
}

func (d *Display) Stop() {
	d.Logger.Debug("Stopping display")
	close(d.stopChan)
	d.resetTerminal()
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
	d.Logger.Debug("Waiting for display to stop")
	select {
	case <-d.quit:
		d.Logger.Debug("Display stopped")
	case <-time.After(5 * time.Second): //nolint:gomnd
		d.Logger.Debug("Timeout waiting for display to stop")
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
func getGoroutineInfo() string {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

func (d *Display) scheduleUpdate() {
	d.updateMutex.Lock()
	defer d.updateMutex.Unlock()

	if !d.updatePending {
		d.updatePending = true
		go func() {
			time.Sleep(250 * time.Millisecond) // Increased delay to reduce update frequency
			d.app.QueueUpdateDraw(func() {
				d.updateMutex.Lock()
				d.updatePending = false
				d.updateMutex.Unlock()

				d.updateDisplay()
			})
		}()
	}
}

func (d *Display) updateDisplay() {
	d.renderTable()
	d.updateLogBox()
	d.logDebugInfo()
}

func (d *Display) renderTable() {
	logDebugf("Rendering table started")

	statuses := make([]*Status, 0, len(d.statuses))
	for _, status := range d.statuses {
		statuses = append(statuses, status)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})

	logDebugf("Statuses sorted")

	// Initialize lastTableState with header row
	d.lastTableState = [][]string{make([]string, len(DisplayColumns))}
	for col, column := range DisplayColumns {
		d.lastTableState[0][col] = column.Text
	}

	for row, status := range statuses {
		tableRow := make([]string, len(DisplayColumns))
		for col, column := range DisplayColumns {
			cellText := column.DataFunc(*status)
			tableRow[col] = cellText
			cell := tview.NewTableCell(cellText).
				SetMaxWidth(column.Width).
				SetTextColor(column.Color)
			d.table.SetCell(row+1, col, cell)
		}
		d.lastTableState = append(d.lastTableState, tableRow)
	}

	logDebugf("Table rendered")
}

func (d *Display) updateLogBox() {
	lines := logger.GetLastLines(d.LogFileName, MaxLogLines)
	d.LogBox.Clear()
	for _, line := range lines {
		fmt.Fprintln(d.LogBox, line)
	}
}

func (d *Display) logDebugInfo() {
	d.Logger.Infof("--- Debug Info ---")
	d.Logger.Infof("Number of statuses: %d", len(d.statuses))
	d.Logger.Infof("LogBox title: %s", d.LogBox.GetTitle())
	d.Logger.Infof("LogBox content length: %d", len(d.LogBox.GetText(true)))
	d.Logger.Infof("LogBuffer size: %d", len(d.logBuffer.GetLines()))
	d.Logger.Infof("------------------")
}
