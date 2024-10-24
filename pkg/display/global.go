package display

import (
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

type GlobalProgammer interface {
	InitProgram(m *DisplayModel) error
	GetProgram() *GlobalProgram
	Quit()
	Run() (tea.Model, error)
}

// GlobalProgram represents the singleton instance
type GlobalProgram struct {
	Program *tea.Program
}

var (
	globalProgramInstance GlobalProgammer
	globalProgramOnce     sync.Once
)

var GetGlobalProgramFunc = GetGlobalProgram

// GetGlobalProgram returns the singleton instance of GlobalProgram
func GetGlobalProgram() GlobalProgammer {
	globalProgramOnce.Do(func() {
		if os.Getenv("ANDAIME_TEST_MODE") == "true" {
			globalProgramInstance = &MockProgram{}
		} else {
			globalProgramInstance = new(GlobalProgram)
		}
	})
	return globalProgramInstance
}

// InitProgram initializes the tea.Program
func (gp *GlobalProgram) InitProgram(m *DisplayModel) error {
	gp.Program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	SetGlobalModel(m)
	return nil
}

// GetProgram returns the tea.Program instance
func (gp *GlobalProgram) GetProgram() *GlobalProgram {
	return gp
}

func (gp *GlobalProgram) Quit() {
	m := GetGlobalModelFunc()
	if m != nil {
		m.quitChan <- true
	}

	gp.Program.Quit()
}

func (gp *GlobalProgram) Run() (tea.Model, error) {
	if os.Getenv("TEST_MODE") == "true" {
		gp.Program = tea.NewProgram(
			GetGlobalModelFunc(),
			tea.WithoutRenderer(),
		)
	}
	return gp.Program.Run()
}
