package display

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

const (
	NumberOfLogLinesToDisplay = 20
	updateInterval            = time.Second
)

type ColumnDef struct {
	Header string
	Width  int
	Getter func(interface{}) string
}

type CommandDisplay struct {
	app          *tview.Application
	table        *tview.Table
	logView      *tview.TextView
	progressBar  *tview.TextView
	columns      []ColumnDef
	data         *sync.Map
	logger       *zap.Logger
	logFile      string
	updateTicker *time.Ticker
	progress     *atomic.Int64
	total        int64
	quit         chan struct{}
	quitHandler  func()
}

func NewCommandDisplay(title string, columns []ColumnDef, data *sync.Map, logFile string, total int64) *CommandDisplay {
	cd := &CommandDisplay{
		app:         tview.NewApplication(),
		table:       tview.NewTable().SetBorders(true),
		logView:     tview.NewTextView().SetScrollable(true).SetDynamicColors(true),
		progressBar: tview.NewTextView().SetDynamicColors(true),
		columns:     columns,
		data:        data,
		logFile:     logFile,
		progress:    &atomic.Int64{},
		total:       total,
		quit:        make(chan struct{}),
	}

	cd.setupLogger()
	cd.setupTable(title)
	cd.setupLayout()

	return cd
}

func (cd *CommandDisplay) setupLogger() {
	logger, _ := zap.NewDevelopment(zap.AddCaller())
	cd.logger = logger
}

func (cd *CommandDisplay) setupTable(title string) {
	cd.table.SetTitle(title).SetTitleAlign(tview.AlignLeft)
	for i, col := range cd.columns {
		cd.table.SetCell(0, i, tview.NewTableCell(col.Header).SetSelectable(false))
	}
}

func (cd *CommandDisplay) setupLayout() {
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cd.progressBar, 1, 0, false).
		AddItem(cd.table, 0, 1, false).
		AddItem(tview.NewTextView().SetText("Log:"), 1, 0, false).
		AddItem(cd.logView, 0, 1, false)

	cd.app.SetRoot(flex, true)
}

func (cd *CommandDisplay) Start() {
	cd.updateTicker = time.NewTicker(updateInterval)
	go cd.runUpdateLoop()
	cd.setupInputCapture()
	if err := cd.app.Run(); err != nil {
		cd.logger.Error("Error running display", zap.Error(err))
	}
}

func (cd *CommandDisplay) runUpdateLoop() {
	for range cd.updateTicker.C {
		cd.app.QueueUpdateDraw(func() {
			cd.updateTable()
			cd.updateLog()
			cd.renderProgress()
		})
	}
}

func (cd *CommandDisplay) setupInputCapture() {
	cd.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			if cd.quitHandler != nil {
				cd.quitHandler()
			}
			return nil
		}
		return event
	})
}

func (cd *CommandDisplay) Stop() {
	cd.updateTicker.Stop()
	cd.app.Stop()
}

func (cd *CommandDisplay) updateTable() {
	cd.data.Range(func(key, value interface{}) bool {
		row := cd.table.GetRowCount()
		for col, colDef := range cd.columns {
			cd.table.SetCell(row, col, tview.NewTableCell(colDef.Getter(value)).SetMaxWidth(colDef.Width))
		}
		return true
	})
}

func (cd *CommandDisplay) updateLog() {
	content, err := os.ReadFile(cd.logFile)
	if err != nil {
		cd.logger.Error("Failed to read log file", zap.Error(err))
		return
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > NumberOfLogLinesToDisplay {
		lines = lines[len(lines)-NumberOfLogLinesToDisplay:]
	}
	logText := strings.Join(lines, "\n")
	logText += fmt.Sprintf("\n\nTo view full log: tail -f %s", cd.logFile)
	cd.logView.SetText(logText)
}

func (cd *CommandDisplay) UpdateProgress(progress int64) {
	cd.progress.Store(progress)
}

func (cd *CommandDisplay) renderProgress() {
	progress := cd.progress.Load()
	percentage := float64(progress) / float64(cd.total) * 100 //nolint:gomnd
	cd.progressBar.SetText(fmt.Sprintf("[yellow]Progress: [green]%.2f%% [yellow](%d/%d)", percentage, progress, cd.total))
}

func (cd *CommandDisplay) ShouldQuit() bool {
	select {
	case <-cd.quit:
		return true
	default:
		return false
	}
}

func (cd *CommandDisplay) SetQuitHandler(handler func()) {
	cd.quitHandler = handler
}
