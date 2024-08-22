package display

import (
	"sync"

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

func (gp *GlobalProgram) Quit() {
	m := GetGlobalModel()
	if m != nil {
		m.quitChan <- true
	}

	gp.program.Quit()
}

func (gp *GlobalProgram) Run() (tea.Model, error) {
	return gp.program.Run()
}
