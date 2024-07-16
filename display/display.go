package display

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/logger"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const NumberOfCyclesToHighlight = 8
const HighlightTimer = 250 * time.Millisecond
const HighlightColor = tcell.ColorDarkGreen

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
	testMode           bool
}

type DisplayColumn struct {
	Text     string
	Width    int
	Color    tcell.Color
	DataFunc func(status Status) string
}

var DisplayColumns = []DisplayColumn{
	{Text: "ID", Width: 10, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.ID }},
	{Text: "Type", Width: 10, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.Type }},
	{Text: "Region", Width: 15, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.Region }},
	{Text: "Zone", Width: 15, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.Zone }},
	{Text: "Status", Width: 30, Color: tcell.ColorRed, DataFunc: func(status Status) string { return fmt.Sprintf("%s (%s)", status.Status, status.DetailedStatus) }},
	{Text: "Elapsed", Width: 10, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.ElapsedTime.Round(time.Second).String() }},
	{Text: "Instance ID", Width: 20, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.InstanceID }},
	{Text: "Public IP", Width: 15, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.PublicIP }},
	{Text: "Private IP", Width: 15, Color: tcell.ColorRed, DataFunc: func(status Status) string { return status.PrivateIP }},
}

func NewDisplay(totalTasks int) *Display {
	return newDisplayInternal(totalTasks, false)
}

func NewTestDisplay(totalTasks int) *Display {
	return newDisplayInternal(totalTasks, true)
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
	}

	d.setupTable()
	d.setupLayout()
	d.DebugLog = *logger.Get()

	return d
}

func (d *Display) setupTable() {
	d.table.SetFixed(1, len(DisplayColumns))
	d.table.SetBordersColor(tcell.ColorWhite)

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
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.table, 0, 1, true)

	d.app.SetRoot(flex, true).EnableMouse(false)
}

func (d *Display) UpdateStatus(status *Status) {
	d.statusesMu.RLock()
	existingStatus, exists := d.statuses[status.ID]
	d.statusesMu.RUnlock()

	newStatus := *status // Create a copy of the status
	if !exists {
		d.statusesMu.Lock()
		d.completedTasks++
		newStatus.HighlightCycles = d.fadeSteps
		d.statuses[newStatus.ID] = &newStatus
		d.statusesMu.Unlock()
	} else if *existingStatus != newStatus {
		d.statusesMu.Lock()
		newStatus.HighlightCycles = d.fadeSteps
		d.statuses[newStatus.ID] = &newStatus
		d.statusesMu.Unlock()
	}

	d.app.QueueUpdateDraw(func() {
		d.renderTable()
	})
}

func (d *Display) getHighlightColor(cycles int) tcell.Color {
	if cycles <= 0 {
		return tcell.ColorDefault
	}

	// Start with dark green
	baseR, baseG, baseB := uint8(0), uint8(100), uint8(0)
	// End with white
	targetR, targetG, targetB := uint8(255), uint8(255), uint8(255)

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
	go func() {
		ticker := time.NewTicker(HighlightTimer)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.app.QueueUpdateDraw(func() {
					d.renderTable()
				})
			case <-d.stopChan:
				return
			}
		}
	}()
}

func (d *Display) renderTable() {
	d.statusesMu.RLock()
	statusesCopy := make(map[string]*Status, len(d.statuses))
	for id, status := range d.statuses {
		statusCopy := *status
		statusesCopy[id] = &statusCopy
	}
	d.statusesMu.RUnlock()

	statuses := make([]*Status, 0, len(statusesCopy))
	for _, status := range statusesCopy {
		statuses = append(statuses, status)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})

	d.lastTableState = make([][]string, len(statuses)+1)
	d.lastTableState[0] = make([]string, len(DisplayColumns))
	for col, header := range DisplayColumns {
		d.lastTableState[0][col] = header.Text
	}

	if !d.testMode {
		for row, status := range statuses {
			highlightColor := d.getHighlightColor(status.HighlightCycles)
			d.lastTableState[row+1] = make([]string, len(DisplayColumns))
			for col, column := range DisplayColumns {
				cellText := column.DataFunc(*status)
				paddedText := d.padText(cellText, column.Width)
				cell := tview.NewTableCell(paddedText).
					SetMaxWidth(column.Width).
					SetTextColor(highlightColor)
				d.table.SetCell(row+1, col, cell)
				d.lastTableState[row+1][col] = cellText
			}
		}
	}

	d.statusesMu.Lock()
	for id, status := range d.statuses {
		if status.HighlightCycles > 0 {
			status.HighlightCycles--
			d.statuses[id] = status
		}
	}
	d.statusesMu.Unlock()
}

func (d *Display) padText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}

func (d *Display) Start(sigChan chan os.Signal) {
	if d.testMode {
		// Simulate running the application without starting tview
		go func() {
			d.startHighlightTimer()
			<-d.stopChan
			close(d.quit)
		}()
		return
	}

	go func() {
		d.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlC {
				sigChan <- os.Interrupt
				return nil
			}
			return event
		})

		d.startHighlightTimer()

		if err := d.app.Run(); err != nil {
			d.DebugLog.Error(fmt.Sprintf("Error running display: %v", err))
		}
	}()

	go func() {
		<-d.stopChan
		d.app.QueueUpdateDraw(func() {
			d.app.Stop()
		})
		close(d.quit)
	}()
}

func (d *Display) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopChan)
		d.WaitForStop()
		d.printFinalTableState()
	})
}

func (d *Display) WaitForStop() {
	<-d.quit
}

func (d *Display) printFinalTableState() {
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
			fmt.Print(strings.Repeat("-", width+2))
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
		printSeparator() // Print separator after each row
	}
}
