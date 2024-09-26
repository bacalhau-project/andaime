package beta

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func RotateServiceStates(i int) models.ServiceState {
	i = i % int(models.ServiceStateUnknown)
	return models.ServiceState(i)
}

func TestDisplayLayout(t *testing.T) {
	// Initialize the display model

	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "test-unique-id")
	m := display.GetGlobalModelFunc()
	// Add test machines
	testMachines := make(map[string]models.Machiner)

	testMachines["test1"] = &models.Machine{
		Name:          "test1",
		CloudProvider: models.DeploymentTypeAzure,
		Type:          models.AzureResourceTypeVM,
		Location:      "us-west-2",
		StatusMessage: "apple grape mango",
		Orchestrator:  true,
		StartTime:     time.Now().Add(-29 * time.Second),
	}
	testMachines["test2"] = &models.Machine{
		Name:          "test2",
		CloudProvider: models.DeploymentTypeAzure,
		Type:          models.AzureResourceTypeVM,
		Location:      "us-west-2",
		StatusMessage: "nectarine fig elderberry",
		Orchestrator:  true,
		StartTime:     time.Now().Add(-29 * time.Second),
	}
	testMachines["test3"] = &models.Machine{
		Name:          "test3",
		CloudProvider: models.DeploymentTypeAzure,
		Type:          models.AzureResourceTypeVM,
		Location:      "us-west-2",
		StatusMessage: "grape quince kiwi",
		Orchestrator:  true,
		StartTime:     time.Now().Add(-29 * time.Second),
	}
	testMachines["test4"] = &models.Machine{
		Name:          "test4",
		CloudProvider: models.DeploymentTypeAzure,
		Type:          models.AzureResourceTypeVM,
		Location:      "us-west-2",
		StatusMessage: "cherry orange quince",
		Orchestrator:  true,
		StartTime:     time.Now().Add(-29 * time.Second),
	}
	testMachines["test5"] = &models.Machine{
		Name:          "test5",
		CloudProvider: models.DeploymentTypeAzure,
		Type:          models.AzureResourceTypeVM,
		Location:      "us-west-2",
		StatusMessage: "raspberry ugli kiwi",
		Orchestrator:  true,
		StartTime:     time.Now().Add(-29 * time.Second),
	}

	m.Deployment.SetMachines(testMachines)

	for _, machine := range m.Deployment.GetMachines() {
		for _, service := range models.RequiredServices {
			machine.SetServiceState(service.Name, models.ServiceStateUnknown)
		}
	}

	i := 0
	for _, machine := range m.Deployment.Machines {
		for _, resource := range machine.GetMachineResources() {
			machine.SetMachineResourceState(resource.ResourceName, models.ResourceStateSucceeded)
		}

		for _, service := range machine.GetServices() {
			machine.SetServiceState(service.Name, RotateServiceStates(i))
			i++
		}
	}

	// Render the table
	renderedTable := m.RenderFinalTable()

	// Split the rendered table into lines
	lines := strings.Split(renderedTable, "\n")

	// Test the overall structure
	assert.GreaterOrEqual(
		t,
		len(lines),
		9,
		"The table should have 9 lines including borders and empty line (may have more for text section)",
	)

	// Test the header
	expectedHeader := "│ "
	for _, column := range display.DisplayColumns {
		expectedHeader += column.Title
		if column.Title != "" {
			expectedHeader += strings.Repeat(" ", column.Width-len(column.Title))
		}
	}
	expectedHeader += "│"

	assert.Equal(t, expectedHeader, lines[1], "Header line should match expected format")

	// Test each machine row
	for _, machine := range testMachines {
		// Get the line (have to do it this way because the line numbers are not consistent)
		line := ""
		for _, putativeLine := range lines {
			if strings.Contains(putativeLine, machine.GetName()) {
				line = putativeLine
				break
			}
		}
		assert.True(t, line != "", "Machine line should be found")

		expectedLineSprintfString := "│ "
		for _, column := range display.DisplayColumns {
			if column.Title != "" {
				expectedLineSprintfString += "%-" + strconv.Itoa(column.Width) + "s"
			} else {
				expectedLineSprintfString += "%0s"
			}
		}
		expectedLineSprintfString += "│"
		// "│ %-9s %-5s %-14s %-30s %-20s %-9s %-14s %-14s %-2s %-2s %-2s %-2s │",

		expectedLine := fmt.Sprintf(
			expectedLineSprintfString,
			machine.GetName(),
			models.AzureResourceTypeVM.ShortResourceName,
			machine.GetLocation(),
			machine.GetStatusMessage(),
			"██████████████████",
			"29.0s",
			machine.GetOrchestratorIP(),
			machine.GetPrivateIP(),
			display.ConvertOrchestratorToEmoji(machine.IsOrchestrator()),
			display.ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeSSH.Name)),
			display.ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeDocker.Name)),
			display.ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeBacalhau.Name)),
			display.ConvertStateToEmoji(machine.GetServiceState(models.ServiceTypeScript.Name)),
			"",
		)
		assert.Equal(
			t,
			expectedLine,
			line,
			fmt.Sprintf("Machine line should match expected format"),
		)
	}
}

func TestColumnWidths(t *testing.T) {
	m := display.GetGlobalModelFunc()
	renderedTable := m.RenderFinalTable()
	lines := strings.Split(renderedTable, "\n")

	// Test column widths
	columnWidths := []int{}
	for _, column := range display.DisplayColumns {
		columnWidths = append(columnWidths, column.Width)
	}
	for i, width := range columnWidths {
		start := 2 // Account for left border and space
		for j := 0; j < i; j++ {
			start += columnWidths[j]
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
	viper.Set("general.project_prefix", "test-project")
	m := display.GetGlobalModelFunc()
	m.Deployment.SetMachine("test1", &models.Machine{
		Name:          "test1",
		CloudProvider: models.DeploymentTypeAzure,
		Type:          models.AzureResourceTypeVM,
		Location:      "us-west-2",
		StatusMessage: "test",
		Orchestrator:  true,
	})
	for _, machine := range m.Deployment.Machines {
		for _, resource := range machine.GetMachineResources() {
			machine.SetMachineResourceState(resource.ResourceName, models.ResourceStateSucceeded)
		}
	}
	renderedTable := m.RenderFinalTable()
	lines := strings.Split(renderedTable, "\n")

	startOfProgressBar := 0 // For the start of the table
	progressBarIndex := 0
	for i, column := range display.DisplayColumns {
		startOfProgressBar += column.Width + 1
		if column.Title == "Progress" {
			progressBarIndex = i
			break
		}
	}

	progressBarStart := startOfProgressBar
	progressBarEnd := progressBarStart + display.DisplayColumns[progressBarIndex].Width
	progressBar := lines[2][progressBarStart:progressBarEnd]

	assert.Equal(
		t,
		18,
		len(strings.Trim(progressBar, " \xe2\x96")),
		"Progress bar should be 18 filled blocks",
	)
}

// func TestTimeFormat(t *testing.T) {
// 	m := display.GetGlobalModelFunc()
// 	m.Deployment.Machines = []models.Machine{
// 		{
// 			Name:          "test",
// 			Type:          models.AzureResourceTypeVM,
// 			Location:      "us-west-2",
// 			StatusMessage: "test",
// 			Progress:      display.AzureTotalSteps,
// 			Orchestrator:  true,
// 			StartTime:     time.Now().Add(-29 * time.Second),
// 		},
// 	}
// 	renderedTable := m.RenderFinalTable()
// 	lines := strings.Split(renderedTable, "\n")

// 	assert.Contains(t, lines[2], "29.0s", "Time should be formatted as '29.0s'")
// }

// func TestEmojiColumns(t *testing.T) {
// 	m := display.GetGlobalModelFunc()
// 	m.Deployment.Machines = []models.Machine{
// 		{
// 			Name:          "test",
// 			Type:          models.AzureResourceTypeVM,
// 			Location:      "us-west-2",
// 			StatusMessage: "test",
// 			Progress:      3,
// 			Orchestrator:  true,
// 			SSH:           models.ServiceStateFailed,
// 			Docker:        models.ServiceStateSucceeded,
// 			Bacalhau:      models.ServiceStateNotStarted,
// 		},
// 	}
// 	renderedTable := m.RenderFinalTable()
// 	lines := strings.Split(renderedTable, "\n")

// 	assert.Contains(
// 		t,
// 		lines[2],
// 		fmt.Sprintf(
// 			"%s  %s  %s  %s",
// 			display.ConvertOrchestratorToEmoji(true),
// 			display.ConvertStateToEmoji(models.ServiceStateFailed),
// 			display.ConvertStateToEmoji(models.ServiceStateSucceeded),
// 			display.ConvertStateToEmoji(models.ServiceStateNotStarted),
// 		),
// 		"Emoji columns should match expected format",
// 	)
// }
