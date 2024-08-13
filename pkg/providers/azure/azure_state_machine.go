package azure

import (
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
)

import (
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
)

// Add this function at the end of the file
func UpdateStatus(status *models.Status) {
	display.UpdateStatus(status)
}
