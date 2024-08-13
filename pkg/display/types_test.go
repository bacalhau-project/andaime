package display

import (
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestUpdateGlobalStatus(t *testing.T) {
	// Initialize the global status map
	GlobalStatusMap = make(map[string]*models.Status)

	// Create a test status
	testStatus := &models.Status{
		ID:       "test1",
		Type:     "test",
		Location: "us-west-2",
		Status:   "Running",
	}

	// Update the global status
	UpdateGlobalStatus(testStatus)

	// Check if the status was updated correctly
	assert.Len(t, GlobalStatusMap, 1)
	assert.Equal(t, testStatus, GlobalStatusMap["test1"])
}
