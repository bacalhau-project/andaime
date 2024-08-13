package azure

import (
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
)

// UpdateStatus updates the status using the display package
func UpdateStatus(status *models.Status) {
	display.UpdateStatus(status)
}
