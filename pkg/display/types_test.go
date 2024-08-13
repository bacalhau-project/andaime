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
		Type:     models.UpdateStatusResourceTypeVM,
		Location: "us-west-2",
		Status:   "Running",
	}

	// Update the status
	updatedModel, _ := model.Update(models.StatusUpdateMsg{Status: testStatus})

	// Assert that the model was updated
	updatedDisplayModel, ok := updatedModel.(*DisplayModel)
	assert.True(t, ok, "Updated model should be of type *DisplayModel")

	// Check if the status was updated correctly
	assert.Len(t, updatedDisplayModel.Deployment.Machines, 1)
	assert.Equal(t, testStatus.Status, updatedDisplayModel.Deployment.Machines[0].Status)
	assert.Equal(t, testStatus.ID, updatedDisplayModel.Deployment.Machines[0].ID)
	assert.Equal(t, testStatus.Location, updatedDisplayModel.Deployment.Machines[0].Location)
	assert.Equal(t, string(testStatus.Type), updatedDisplayModel.Deployment.Machines[0].Type)
}
