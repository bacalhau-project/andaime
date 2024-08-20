package display

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"runtime"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Constants
const (
	LogLines           = 10
	AzureTotalSteps    = 7
	StatusLength       = 30
	TickerInterval     = 100 * time.Millisecond
	ProgressBarPadding = 2
)

// DisplayColumn represents a column in the display table
type DisplayColumn struct {
	Title       string
	Width       int
	Height      int
	EmojiColumn bool
}

// DisplayColumns defines the structure of the display table
//
//nolint:gomnd
var DisplayColumns = []DisplayColumn{
	{Title: "Name", Width: 10},
	{Title: "Type", Width: 6},
	{Title: "Location", Width: 16},
	{Title: "Status", Width: StatusLength},
	{Title: "Progress", Width: 20},
	{Title: "Time", Width: 8},
	{Title: "Pub IP", Width: 19},
	{Title: "Priv IP", Width: 19},
	{Title: models.DisplayTextOrchestrator, Width: 2, EmojiColumn: true},
	{Title: models.DisplayTextSSH, Width: 2, EmojiColumn: true},
	{Title: models.DisplayTextDocker, Width: 2, EmojiColumn: true},
	{Title: models.DisplayTextBacalhau, Width: 2, EmojiColumn: true},
	{Title: "", Width: 1},
}

func (m *DisplayModel) updateLogCmd() tea.Cmd {
	return func() tea.Msg {
		return logLinesMsg(logger.GetLastLines(LogLines))
	}
}

func (m *DisplayModel) updateMachineStatus(machine *models.Machine, status *models.DisplayStatus) {
	l := logger.Get()
	if status.StatusMessage != "" {
		trimmedStatus := strings.TrimSpace(status.StatusMessage)
		if len(trimmedStatus) > StatusLength-3 {
			l.Debugf("Status too long, truncating: '%s'", trimmedStatus)
			machine.StatusMessage = trimmedStatus[:StatusLength-3] + "…"
		} else {
			machine.StatusMessage = fmt.Sprintf("%-*s", StatusLength, trimmedStatus)
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
	if status.ElapsedTime > 0 && !machine.Complete() {
		machine.ElapsedTime = status.ElapsedTime
	}
	if status.Orchestrator {
		machine.Orchestrator = status.Orchestrator
	}

	if status.SSH != models.ServiceStateUnknown {
		sshService := machine.MachineServices["SSH"]
		sshService.State = status.SSH
		machine.MachineServices["SSH"] = sshService
	}
	if status.Docker != models.ServiceStateUnknown {
		dockerService := machine.MachineServices["Docker"]
		dockerService.State = status.Docker
		machine.MachineServices["Docker"] = dockerService
	}
	if status.CorePackages != models.ServiceStateUnknown {
		corePackagesService := machine.MachineServices["CorePackages"]
		corePackagesService.State = status.CorePackages
		machine.MachineServices["CorePackages"] = corePackagesService
	}
	if status.Bacalhau != models.ServiceStateUnknown {
		bacalhauService := machine.MachineServices["Bacalhau"]
		bacalhauService.State = status.Bacalhau
		machine.MachineServices["Bacalhau"] = bacalhauService
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
			StartTime: time.Now(),
		}
		if status.Name != "" {
			newMachine.Name = status.Name
		}
		if status.Type != (models.AzureResourceTypes{}) {
			newMachine.Type = status.Type
		}
		if status.Location != "" {
			newMachine.Location = status.Location
		}
		if status.StatusMessage != "" {
			newMachine.StatusMessage = status.StatusMessage
		}
		m.Deployment.Machines = append(m.Deployment.Machines, newMachine)
		return &m.Deployment.Machines[len(m.Deployment.Machines)-1], false
	}

	return nil, false
}

func AggregateColumnWidths() int {
	width := 0
	for _, column := range DisplayColumns {
		width += column.Width
	}
	return width
}

// DisplayModel represents the main display model
type DisplayModel struct {
	Deployment       *models.Deployment
	TextBox          []string
	Quitting         bool
	LastUpdate       time.Time
	DebugMode        bool
	UpdateTimes      []time.Duration
	UpdateTimesIndex int
	UpdateTimesSize  int
	LastUpdateStart  time.Time
	CPUUsage         float64
	MemoryUsage      uint64
	BatchedUpdates   []models.StatusUpdateMsg
	BatchUpdateTimer *time.Timer
	quitChan         chan bool
	goroutineCount   int64
	keyEventChan     chan tea.KeyMsg
	logger           *logger.Logger
	activeGoroutines sync.Map
}

func (m *DisplayModel) RegisterGoroutine(label string) int64 {
	l := logger.Get()
	id := atomic.AddInt64(&m.goroutineCount, 1)
	m.activeGoroutines.Store(id, label)
	l.Debugf("Goroutine started: %s (ID: %d)", label, id)
	return id
}

func (m *DisplayModel) DeregisterGoroutine(id int64) {
	l := logger.Get()
	if label, ok := m.activeGoroutines.LoadAndDelete(id); ok {
		l.Debugf("Goroutine finished: %s (ID: %d)", label, id)
	}
	atomic.AddInt64(&m.goroutineCount, -1)
}

// DisplayMachine represents a single machine in the deployment
type DisplayMachine struct {
	Name          string
	Type          models.AzureResourceTypes
	Location      string
	StatusMessage string
	StartTime     time.Time
	ElapsedTime   time.Duration
	PublicIP      string
	PrivateIP     string
	Orchestrator  bool
	SSH           models.ServiceState
	Docker        models.ServiceState
	CorePackages  models.ServiceState
	Bacalhau      models.ServiceState
}

var (
	globalModelInstance *DisplayModel
	globalModelOnce     sync.Once
)

var GetGlobalModelFunc func() *DisplayModel = GetGlobalModel

// GetGlobalModel returns the singleton instance of DisplayModel
func GetGlobalModel() *DisplayModel {
	if globalModelInstance == nil {
		globalModelOnce.Do(func() {
			globalModelInstance = InitialModel()
		})
	}
	return globalModelInstance
}

func SetGlobalModel(m *DisplayModel) {
	globalModelInstance = m
}

// InitialModel creates and returns a new DisplayModel
func InitialModel() *DisplayModel {
	model := &DisplayModel{
		Deployment:       models.NewDeployment(),
		TextBox:          []string{"Resource Status Monitor"},
		LastUpdate:       time.Now(),
		DebugMode:        os.Getenv("DEBUG_DISPLAY") == "1",
		UpdateTimes:      make([]time.Duration, 100),
		UpdateTimesIndex: 0,
		UpdateTimesSize:  100,
		quitChan:         make(chan bool),
		keyEventChan:     make(chan tea.KeyMsg),
		logger:           logger.Get(),
	}
	go model.handleKeyEvents()
	return model
}

func (m *DisplayModel) handleKeyEvents() {
	l := logger.Get()
	for {
		select {
		case <-m.quitChan:
			l.Debug("Quit signal received in handleKeyEvents")
			return
		case key := <-m.keyEventChan:
			if (key.String() == "q" || key.String() == "ctrl+c") && !m.Quitting {
				m.Quitting = true
				l.Debugf(
					"Quit command received (q or ctrl+c) at %s",
					time.Now().Format(time.RFC3339Nano),
				)
				close(m.quitChan)
				l.Debug("Quit channel closed")
				return
			}
		}
	}
}

// Init initializes the DisplayModel
func (m *DisplayModel) Init() tea.Cmd {
	return m.tickCmd()
}

// Update handles updates to the DisplayModel
func (m *DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	l := logger.Get()

	if m.Quitting {
		l.Debug("Model is quitting, returning tea.Quit")
		return m, tea.Quit
	}

	// Handle key events directly in the Update method
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		keyPressTime := time.Now()
		l.Debugf(
			"Key pressed at %s: %s",
			keyPressTime.Format(time.RFC3339Nano),
			keyMsg.String(),
		)
		if keyMsg.Type == tea.KeyCtrlC || keyMsg.String() == "q" {
			l.Debug("Quit command received in Update")
			m.Quitting = true
			close(m.quitChan)
			return m, tea.Quit
		}
	}

	updateStart := time.Now()
	defer func() {
		updateDuration := time.Since(updateStart)
		m.UpdateTimes[m.UpdateTimesIndex] = updateDuration
		m.UpdateTimesIndex = (m.UpdateTimesIndex + 1) % m.UpdateTimesSize
	}()

	switch msg.(type) {
	case tickMsg:
		return m, tea.Batch(m.tickCmd(), m.updateLogCmd(), m.applyBatchedUpdatesCmd())
	case batchedUpdatesAppliedMsg:
		m.BatchUpdateTimer = nil
	}

	// Update CPU and memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.MemoryUsage = memStats.Alloc
	m.CPUUsage = getCPUUsage()

	return m, tea.Batch(m.tickCmd(), m.updateLogCmd())
}

func (m *DisplayModel) applyBatchedUpdatesCmd() tea.Cmd {
	l := logger.Get()
	return func() tea.Msg {
		atomic.AddInt64(&m.goroutineCount, 1)
		defer atomic.AddInt64(&m.goroutineCount, -1)

		if m.Quitting {
			l.Debug("Quitting, skipping batch updates")
			return tea.Quit
		}

		m.applyBatchedUpdates()
		return batchedUpdatesAppliedMsg{}
	}
}

func (m *DisplayModel) applyBatchedUpdates() {
	for _, update := range m.BatchedUpdates {
		m.UpdateStatus(update.Status)
	}
	m.BatchedUpdates = nil

	// Check if all machines have completed their deployment and Docker/Core Packages installation
	allCompleted := true
	allDockerAndCorePackagesInstalled := true
	for _, machine := range m.Deployment.Machines {
		progress, total := machine.ResourcesComplete()
		if progress != total ||
			machine.MachineServices["SSH"].State != models.ServiceStateSucceeded {
			allCompleted = false
			break
		}
		if machine.MachineServices["Docker"].State != models.ServiceStateSucceeded ||
			machine.MachineServices["CorePackages"].State != models.ServiceStateSucceeded {
			allDockerAndCorePackagesInstalled = false
			break
		}
	}

	// If all machines are completed and Docker/Core Packages are installed, install Bacalhau
	if allCompleted && allDockerAndCorePackagesInstalled {
		orchestratorInstalled := false
		for _, machine := range m.Deployment.Machines {
			if machine.Orchestrator &&
				machine.MachineServices["Bacalhau"].State != models.ServiceStateSucceeded {
				bacalhauService := machine.MachineServices["Bacalhau"]
				bacalhauService.State = models.ServiceStateUpdating
				machine.MachineServices["Bacalhau"] = bacalhauService

				// TODO: Implement Bacalhau orchestrator installation

				bacalhauService.State = models.ServiceStateSucceeded
				machine.MachineServices["Bacalhau"] = bacalhauService
				orchestratorInstalled = true
				break
			}
		}

		if orchestratorInstalled {
			for _, machine := range m.Deployment.Machines {
				if !machine.Orchestrator &&
					machine.MachineServices["Bacalhau"].State != models.ServiceStateSucceeded {
					bacalhauService := machine.MachineServices["Bacalhau"]
					bacalhauService.State = models.ServiceStateUpdating
					machine.MachineServices["Bacalhau"] = bacalhauService
					// TODO: Implement Bacalhau worker installation
					bacalhauService.State = models.ServiceStateSucceeded
					machine.MachineServices["Bacalhau"] = bacalhauService
				}
			}
		}
	}
}

type batchedUpdatesAppliedMsg struct{}

func getCPUUsage() float64 {
	var startTime time.Time
	var startUsage float64
	startTime = time.Now()
	startUsage, _ = getCPUTime()
	time.Sleep(100 * time.Millisecond)
	endTime := time.Now()
	endUsage, _ := getCPUTime()

	cpuUsage := (endUsage - startUsage) / endTime.Sub(startTime).Seconds()
	return cpuUsage * 100 // Return as percentage
}

func getCPUTime() (float64, error) {
	contents, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			var total float64
			for _, field := range fields[1:] {
				val, _ := strconv.ParseFloat(field, 64)
				total += val
			}
			return total, nil
		}
	}
	return 0, fmt.Errorf("CPU info not found")
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

	var avgUpdateTime time.Duration
	var totalUpdates int
	sum := int64(0)
	for _, d := range m.UpdateTimes {
		if d != 0 {
			sum += d.Nanoseconds()
			totalUpdates++
		}
	}
	if totalUpdates > 0 {
		avgUpdateTime = time.Duration(sum / int64(totalUpdates))
	}

	performanceInfo := fmt.Sprintf(
		"Avg Update Time: %v, CPU Usage: %.2f%%, Memory Usage: %d MB, Circular Buffer Size: %d, Goroutines: %d",
		avgUpdateTime,
		m.CPUUsage,
		m.MemoryUsage/1024/1024,
		m.UpdateTimesSize,
		atomic.LoadInt64(&m.goroutineCount),
	)

	// logger.WriteProfileInfo(performanceInfo)

	// profileFilePath := logger.GetProfileFilePath()
	// profileFileInfo := fmt.Sprintf("Profile information written to: %s", profileFilePath)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		textBoxStyle.Render(logContent),
		infoStyle.Render(infoText),
		infoStyle.Render(performanceInfo),
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
		tableStr += strings.Repeat("-", AggregateColumnWidths()) + "\n"
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

func (m *DisplayModel) getMachineRowData(machine models.Machine) []string {
	elapsedTime := time.Since(machine.StartTime).Truncate(TickerInterval)
	progress, total := machine.ResourcesComplete()
	progressBar := renderProgressBar(
		progress,
		total,
		DisplayColumns[4].Width-ProgressBarPadding,
	)

	return []string{
		machine.Name,
		machine.Type.ShortResourceName,
		machine.Location,
		machine.StatusMessage,
		progressBar,
		formatElapsedTime(elapsedTime),
		machine.PublicIP,
		machine.PrivateIP,
		ConvertOrchestratorToEmoji(machine.Orchestrator),
		ConvertStateToEmoji(machine.MachineServices["SSH"].State),
		ConvertStateToEmoji(machine.MachineServices["Docker"].State),
		ConvertStateToEmoji(machine.MachineServices["Bacalhau"].State),
		"",
	}
}

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

func formatElapsedTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds()/100) % 10

	if minutes > 0 {
		return fmt.Sprintf("%dm%02d.%ds", minutes, seconds, tenths)
	}
	return fmt.Sprintf("%2d.%ds", seconds, tenths)
}

type tickMsg time.Time
type logLinesMsg []string

func (m *DisplayModel) tickCmd() tea.Cmd {
	return tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func ConvertOrchestratorToEmoji(orchestrator bool) string {
	orchString := models.DisplayTextWorkerNode
	if orchestrator {
		orchString = models.DisplayTextOrchestratorNode
	}
	return orchString
}

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
