package display

import (
	"testing"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestUpdateStatus(t *testing.T) {
	// Initialize the model
	model := InitialModel()

	// Create a test status
	testStatus := &models.Status{
		ID:       "test1",
		Type:     "test",
		Location: "us-west-2",
		Status:   "Running",
	}

	// Update the status
	model.Update(models.StatusUpdateMsg{Status: testStatus})

	// Check if the status was updated correctly
	assert.Len(t, model.Deployment.Machines, 1)
	assert.Equal(t, testStatus.Status, model.Deployment.Machines[0].Status)
}
