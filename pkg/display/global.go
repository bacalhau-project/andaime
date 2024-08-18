package display

import (
	"sync"

	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
)

// GlobalProgram represents the singleton instance
type GlobalProgram struct {
	program *tea.Program
}

var (
	globalProgramInstance *GlobalProgram
	globalProgramOnce     sync.Once
)

// GetGlobalProgram returns the singleton instance of GlobalProgram
func GetGlobalProgram() *GlobalProgram {
	globalProgramOnce.Do(func() {
		globalProgramInstance = &GlobalProgram{}
	})
	return globalProgramInstance
}

// InitProgram initializes the tea.Program
func (gp *GlobalProgram) InitProgram(m *DisplayModel) {
	gp.program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	SetGlobalModel(m)
}

// GetProgram returns the tea.Program instance
func (gp *GlobalProgram) GetProgram() *GlobalProgram {
	return gp
}

// UpdateStatus updates the status of a deployment or machine
func (gp *GlobalProgram) UpdateStatus(status *models.DisplayStatus) {
	m := GetGlobalModel()
	if m != nil {
		m.UpdateStatus(status)
	}
}

func (gp *GlobalProgram) Quit() {
	gp.program.Quit()
}

func (gp *GlobalProgram) Run() (tea.Model, error) {
	return gp.program.Run()
}
