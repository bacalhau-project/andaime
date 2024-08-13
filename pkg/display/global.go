package display

import (
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
)

var globalProgram *tea.Program

func InitGlobalProgram(model *DisplayModel) {
	globalProgram = tea.NewProgram(model)
}

func GetGlobalProgram() *tea.Program {
	return globalProgram
}

// UpdateStatus updates the status of a deployment or machine
func UpdateStatus(status *models.Status) {
	if globalProgram != nil {
		globalProgram.Send(models.StatusUpdateMsg{Status: status})
	}
}
