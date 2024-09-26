package display

import (
	tea "github.com/charmbracelet/bubbletea"
)

type MockProgram struct{}

func (m *MockProgram) InitProgram(model *DisplayModel) {}
func (m *MockProgram) Quit()                           {}
func (m *MockProgram) Run() (tea.Model, error)         { return nil, nil }
func (m *MockProgram) GetProgram() *GlobalProgram      { return nil }
