package display

import (
	"github.com/bacalhau-project/andaime/pkg/models"
)

// Global status map
var (
	GlobalStatusMap = make(map[string]*models.Status)
)

// GetStatus retrieves a status from the global map
func GetStatus(id string) *models.Status {
	return GlobalStatusMap[id]
}

// UpdateGlobalStatus updates the status in the global map
func UpdateGlobalStatus(status *models.Status) {
	GlobalStatusMap[status.ID] = status
}
