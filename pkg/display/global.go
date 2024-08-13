package display

import (
	"sync"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/gdamore/tcell/v2"
)

var (
	globalDisplay *Display
	once          sync.Once
)

func GetGlobalMockDisplay(screen tcell.Screen) *Display {
	once.Do(func() {
		globalDisplay = NewDisplay()
		globalDisplay.Visible = false
	})
	return globalDisplay
}

// GetGlobalDisplay returns the global Display instance
func GetGlobalDisplay() *Display {
	once.Do(func() {
		globalDisplay = NewDisplay()
	})
	return globalDisplay
}

// UpdateStatus updates the status of a deployment or machine
func UpdateStatus(status *models.Status) {
	GetGlobalDisplay().UpdateStatus(status)
}
