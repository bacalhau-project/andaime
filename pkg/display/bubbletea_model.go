package display

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DisplayColumn struct {
	Title string
	Width int
}

var DisplayColumns = []DisplayColumn{
	{Title: "ID", Width: 13},
	{Title: "Type", Width: 8},
	{Title: "Location", Width: 12},
	{Title: "Status", Width: 20},
	{Title: "Progress", Width: 20},
	{Title: "Time", Width: 10},
	{Title: "Public IP", Width: 15},
	{Title: "Private IP", Width: 15},
}

// DisplayResources is a subset of the underlying Azure resources
type DisplayResources struct {
	id            string
	resType       string
	location      string
	status        string
	progress      int
	total         int
	startTime     time.Time
	completedTime time.Time
	publicIP      string
	privateIP     string
}

type DisplayModel struct {
	Resources []DisplayResources
	TextBox   string
	Quitting  bool
}

func InitialModel() DisplayModel {
	now := time.Now()
	return DisplayModel{
		Resources: []DisplayResources{
			{
				id:        "res-001",
				resType:   "VM",
				location:  "us-west",
				status:    "Provisioning",
				progress:  0,
				total:     10,
				startTime: now,
				publicIP:  "203.0.113.1",
				privateIP: "10.0.0.1",
			},
			{
				id:        "res-002",
				resType:   "Storage",
				location:  "us-east",
				status:    "Running",
				progress:  5,
				total:     10,
				startTime: now,
				publicIP:  "203.0.113.2",
				privateIP: "10.0.0.2",
			},
			{
				id:        "res-003",
				resType:   "Network",
				location:  "eu-central",
				status:    "Stopping",
				progress:  8,
				total:     10,
				startTime: now,
				publicIP:  "203.0.113.3",
				privateIP: "10.0.0.3",
			},
		},
		TextBox: "Resource Status Monitor",
	}
}

func (m DisplayModel) Init() tea.Cmd {
	return tickCmd()
}

func (m DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		if m.Quitting {
			return m, nil
		}
		m.TextBox = fmt.Sprintf("Last Updated: %s", time.Now().Format("15:04:05"))

		// Simulate status and progress changes
		for i := range m.Resources {
			switch m.Resources[i].status {
			case "Provisioning":
				m.Resources[i].progress++
				if m.Resources[i].progress >= m.Resources[i].total {
					m.Resources[i].status = "Running"
					m.Resources[i].progress = m.Resources[i].total
				}
			case "Running":
				if i%2 == 0 {
					m.Resources[i].status = "Stopping"
					m.Resources[i].progress = 0
				}
			case "Stopping":
				m.Resources[i].progress++
				if m.Resources[i].progress >= m.Resources[i].total {
					m.Resources[i].status = "Stopped"
					m.Resources[i].progress = m.Resources[i].total
					m.Resources[i].completedTime = time.Now()
				}
			case "Stopped":
				// Do nothing, keep the progress at 100%
			}
		}

		return m, tickCmd()
	}
	return m, nil
}

func (m DisplayModel) View() string {
	tableStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))
	cellStyle := lipgloss.NewStyle().
		PaddingLeft(1)
	textBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(70)
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	var tableStr string

	// Render headers
	var headerRow string
	for _, col := range DisplayColumns {
		headerRow += headerStyle.Copy().Width(col.Width).MaxWidth(col.Width).Render(col.Title)
	}
	tableStr += headerRow + "\n"

	// Render rows
	for _, res := range m.Resources {
		var rowStr string
		var elapsedTime string
		if !res.completedTime.IsZero() {
			elapsedTime = res.completedTime.Sub(res.startTime).Truncate(time.Second).String()
		} else {
			elapsedTime = time.Since(res.startTime).Truncate(time.Second).String()
		}
		progressBar := renderProgressBar(res.progress, res.total, DisplayColumns[4].Width-2)

		rowData := []string{
			res.id,
			res.resType,
			res.location,
			res.status,
			progressBar,
			elapsedTime,
			res.publicIP,
			res.privateIP,
		}

		for i, cell := range rowData {
			rowStr += cellStyle.Copy().
				Width(DisplayColumns[i].Width).
				MaxWidth(DisplayColumns[i].Width).
				Render(cell)
		}
		tableStr += rowStr + "\n"
	}

	infoText := infoStyle.Render("Press 'q' or Ctrl+C to quit")

	output := lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		textBoxStyle.Render(m.TextBox),
		"",
		infoText,
	)

	if m.Quitting {
		return output + "\n\nProgram exited. CLI is now available below.\n"
	}

	return output
}

func renderProgressBar(progress, total, width int) string {
	if total == 0 {
		return ""
	}
	filledWidth := int(float64(progress) / float64(total) * float64(width))
	emptyWidth := width - filledWidth

	filled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Render(strings.Repeat("█", filledWidth))
	empty := lipgloss.NewStyle().
		Foreground(lipgloss.Color("237")).
		Render(strings.Repeat("█", emptyWidth))

	return filled + empty
}

func (m DisplayModel) UpdateResources(resources []interface{}) {
	for _, resource := range resources {
		if r, ok := resource.(models.Status); ok {
			for i, existingResource := range m.Resources {
				if existingResource.id == r.ID {
					m.Resources[i] = DisplayResources{
						id:            r.ID,
						resType:       string(r.Type),
						location:      r.Location,
						status:        r.Status,
						progress:      0, // You might want to calculate this based on r.DetailedStatus
						total:         100,
						startTime:     r.StartTime,
						completedTime: time.Time{},
						publicIP:      r.PublicIP,
						privateIP:     r.PrivateIP,
					}
					break
				}
			}
		}
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
