package beta

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestDisplayLayout(t *testing.T) {
	// Initialize the display model
	m := display.GetGlobalModel()

	// Add test machines
	testMachines := []models.Machine{
		{
			Name:          "test1",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "apple grape mango",
			Progress:      3,
			Orchestrator:  true,
			SSH:           models.DisplayEmojiFailed,
			Docker:        models.DisplayEmojiSuccess,
			Bacalhau:      models.DisplayEmojiWaiting,
			StartTime:     time.Now().Add(-29 * time.Second),
		},
		{
			Name:          "test2",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "nectarine fig elderberry",
			Progress:      3,
			Orchestrator:  true,
			SSH:           models.DisplayEmojiFailed,
			Docker:        models.DisplayEmojiSuccess,
			Bacalhau:      models.DisplayEmojiWaiting,
			StartTime:     time.Now().Add(-29 * time.Second),
		},
		{
			Name:          "test3",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "grape quince kiwi",
			Progress:      3,
			Orchestrator:  true,
			SSH:           models.DisplayEmojiFailed,
			Docker:        models.DisplayEmojiSuccess,
			Bacalhau:      models.DisplayEmojiWaiting,
			StartTime:     time.Now().Add(-29 * time.Second),
		},
		{
			Name:          "test4",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "cherry orange quince",
			Progress:      3,
			Orchestrator:  true,
			SSH:           models.DisplayEmojiFailed,
			Docker:        models.DisplayEmojiSuccess,
			Bacalhau:      models.DisplayEmojiWaiting,
			StartTime:     time.Now().Add(-29 * time.Second),
		},
		{
			Name:          "test5",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "raspberry ugli kiwi",
			Progress:      3,
			Orchestrator:  true,
			SSH:           models.DisplayEmojiFailed,
			Docker:        models.DisplayEmojiSuccess,
			Bacalhau:      models.DisplayEmojiWaiting,
			StartTime:     time.Now().Add(-29 * time.Second),
		},
	}

	m.Deployment.Machines = testMachines

	// Render the table
	renderedTable := m.RenderFinalTable()

	// Split the rendered table into lines
	lines := strings.Split(renderedTable, "\n")

	// Test the overall structure
	assert.Equal(t, 9, len(lines), "The table should have 9 lines including borders and empty line")

	// Test the header
	expectedHeader := "│ Name      Type  Location       Status                        Progress            Time      Pub IP         Priv IP        O  S  D  B │"
	assert.Equal(t, expectedHeader, lines[1], "Header line should match expected format")

	// Test each machine row
	for i, machine := range testMachines {
		line := lines[i+2]
		expectedLine := fmt.Sprintf(
			"│ %-9s %-5s %-14s %-30s %-20s %-9s %-14s %-14s %-2s %-2s %-2s %-2s │",
			machine.Name,
			machine.Type,
			machine.Location,
			machine.StatusMessage,
			"██████████████████",
			"29.0s",
			"",
			"",
			"⏼",
			"✘",
			"✔",
			"⟳",
		)
		assert.Equal(
			t,
			expectedLine,
			line,
			fmt.Sprintf("Machine line %d should match expected format", i+1),
		)
	}

	// Test the spacing
	for i := 0; i <= 7; i++ {
		assert.Equal(t, 123, len(lines[i]), fmt.Sprintf("Line %d should be 123 characters long", i))
	}

	// Test the borders
	assert.Equal(t, '┌', rune(lines[0][0]), "Top-left corner should be ┌")
	assert.Equal(t, '┐', rune(lines[0][len(lines[0])-1]), "Top-right corner should be ┐")
	assert.Equal(t, '└', rune(lines[7][0]), "Bottom-left corner should be └")
	assert.Equal(t, '┘', rune(lines[7][len(lines[7])-1]), "Bottom-right corner should be ┘")

	// Test the empty line
	expectedEmptyLine := "│                                                                                                                          │"
	assert.Equal(t, expectedEmptyLine, lines[7], "Empty line should match expected format")

	// Test the top and bottom borders
	expectedBorder := "┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐"
	assert.Equal(t, expectedBorder, lines[0], "Top border should match expected format")
	assert.Equal(t, expectedBorder, lines[8], "Bottom border should match expected format")
}

func TestColumnWidths(t *testing.T) {
	m := display.GetGlobalModel()
	renderedTable := m.RenderFinalTable()
	lines := strings.Split(renderedTable, "\n")

	// Test column widths
	columnWidths := []int{10, 6, 15, 30, 20, 10, 15, 15, 2, 2, 2, 2}
	for i, width := range columnWidths {
		start := 2 // Account for left border and space
		for j := 0; j < i; j++ {
			start += columnWidths[j] + 1 // Add 1 for the space between columns
		}
		end := start + width
		column := lines[1][start:end]
		assert.Equal(
			t,
			width,
			len(column),
			fmt.Sprintf("Column %d should have width %d", i+1, width),
		)
	}
}

func TestProgressBar(t *testing.T) {
	m := display.GetGlobalModel()
	m.Deployment.Machines = []models.Machine{
		{
			Name:          "test",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "test",
			Progress:      3,
			Orchestrator:  true,
		},
	}
	renderedTable := m.RenderFinalTable()
	lines := strings.Split(renderedTable, "\n")

	progressBarStart := 67 // Position where progress bar starts
	progressBarEnd := 87   // Position where progress bar ends
	progressBar := lines[2][progressBarStart:progressBarEnd]

	assert.Equal(t, "██████████████████", progressBar, "Progress bar should be 18 filled blocks")
}

func TestTimeFormat(t *testing.T) {
	m := display.GetGlobalModel()
	m.Deployment.Machines = []models.Machine{
		{
			Name:          "test",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "test",
			Progress:      3,
			Orchestrator:  true,
			StartTime:     time.Now().Add(-29 * time.Second),
		},
	}
	renderedTable := m.RenderFinalTable()
	lines := strings.Split(renderedTable, "\n")

	timeStart := 87 // Position where time starts
	timeEnd := 97   // Position where time ends
	timeString := strings.TrimSpace(lines[2][timeStart:timeEnd])

	assert.Equal(t, "29.0s", timeString, "Time should be formatted as '29.0s'")
}

func TestEmojiColumns(t *testing.T) {
	m := display.GetGlobalModel()
	m.Deployment.Machines = []models.Machine{
		{
			Name:          "test",
			Type:          models.AzureResourceTypeVM,
			Location:      "us-west-2",
			StatusMessage: "test",
			Progress:      3,
			Orchestrator:  true,
			SSH:           models.DisplayEmojiFailed,
			Docker:        models.DisplayEmojiSuccess,
			Bacalhau:      models.DisplayEmojiWaiting,
		},
	}
	renderedTable := m.RenderFinalTable()
	lines := strings.Split(renderedTable, "\n")

	emojiStart := 115 // Position where emoji columns start
	emojiEnd := 123   // Position where emoji columns end
	emojiString := lines[2][emojiStart:emojiEnd]

	assert.Equal(t, "⏼  ✘  ✔  ⟳ ", emojiString, "Emoji columns should match expected format")
}
