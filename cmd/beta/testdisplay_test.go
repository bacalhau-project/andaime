package beta

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestDisplayLayout(t *testing.T) {
	// Initialize the display model
	m := display.GetGlobalModel()

	// Add test machines
	testMachines := []models.Machine{
		{Name: "test1", Type: "test", Location: "us-west-2", Status: "apple grape mango", Progress: 3, Orchestrator: true, SSH: models.DisplayEmojiFailed, Docker: models.DisplayEmojiSuccess, Bacalhau: models.DisplayEmojiWaiting},
		{Name: "test2", Type: "test", Location: "us-west-2", Status: "nectarine fig elderberry", Progress: 4, Orchestrator: false, SSH: models.DisplayEmojiFailed, Docker: models.DisplayEmojiSuccess, Bacalhau: models.DisplayEmojiWaiting},
		{Name: "test3", Type: "test", Location: "us-west-2", Status: "grape quince kiwi", Progress: 5, Orchestrator: false, SSH: models.DisplayEmojiFailed, Docker: models.DisplayEmojiSuccess, Bacalhau: models.DisplayEmojiWaiting},
		{Name: "test4", Type: "test", Location: "us-west-2", Status: "cherry orange quince", Progress: 6, Orchestrator: false, SSH: models.DisplayEmojiFailed, Docker: models.DisplayEmojiSuccess, Bacalhau: models.DisplayEmojiWaiting},
		{Name: "test5", Type: "test", Location: "us-west-2", Status: "raspberry ugli kiwi", Progress: 7, Orchestrator: false, SSH: models.DisplayEmojiFailed, Docker: models.DisplayEmojiSuccess, Bacalhau: models.DisplayEmojiWaiting},
	}

	m.Deployment.Machines = testMachines

	// Render the table
	renderedTable := m.RenderFinalTable()

	// Split the rendered table into lines
	lines := strings.Split(renderedTable, "\n")

	// Test the overall structure
	assert.Equal(t, 9, len(lines), "The table should have 9 lines including borders and empty line")

	// Test the header
	assert.Contains(t, lines[1], "Name      Type  Location       Status                        Progress            Time      Pub IP         Priv IP        O S D B")

	// Test each machine row
	for i, machine := range testMachines {
		line := lines[i+2]
		assert.Contains(t, line, machine.Name)
		assert.Contains(t, line, machine.Type)
		assert.Contains(t, line, machine.Location)
		assert.Contains(t, line, machine.Status)
		assert.Contains(t, line, "█")
		if machine.Orchestrator {
			assert.Contains(t, line, "⏼ ✘ ✔ ⟳")
		} else {
			assert.Contains(t, line, "◯ ✘ ✔ ⟳")
		}
	}

	// Test the spacing
	for i := 1; i <= 7; i++ {
		assert.Equal(t, 123, len(lines[i]), fmt.Sprintf("Line %d should be 123 characters long", i))
	}

	// Test the borders
	assert.Equal(t, '┌', rune(lines[0][0]))
	assert.Equal(t, '┐', rune(lines[0][len(lines[0])-1]))
	assert.Equal(t, '└', rune(lines[7][0]))
	assert.Equal(t, '┘', rune(lines[7][len(lines[7])-1]))

	// Test the empty line
	assert.Equal(t, "│                                                                                                                          │", lines[7])
}
