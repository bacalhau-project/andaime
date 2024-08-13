package display

import (
	"sync"

	"github.com/bacalhau-project/andaime/pkg/models"
)

var (
	once sync.Once
)

// UpdateStatus updates the status of a deployment or machine
func UpdateStatus(status *models.Status) {
	UpdateGlobalStatus(status)
}
