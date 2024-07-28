package display

import (
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestTestDisplayStart(t *testing.T) {
	display := NewTestDisplay(1)
	sigChan := make(chan os.Signal, 1)

	// Start the display in a goroutine
	go display.Start(sigChan)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Update status
	status := &models.Status{ID: "test1", Type: "test", Location: "us-west-2", Status: "Running"}
	display.UpdateStatus(status)

	// Stop the display
	display.Stop()

	// Wait for it to stop
	display.WaitForStop()

	// Assert that no panic occurred and the status was updated
	assert.NotNil(t, display.Logger)
	assert.Len(t, display.Statuses, 1)
	assert.Equal(t, status.ID, display.Statuses["test1"].ID)
}

func TestTestDisplayUpdateStatus(t *testing.T) {
	display := NewTestDisplay(1)
	sigChan := make(chan os.Signal, 1)

	// Start the display
	go display.Start(sigChan)
	defer display.Stop()

	// Update status multiple times
	for i := 0; i < 10; i++ {
		status := &models.Status{
			ID:       "test1",
			Type:     "test",
			Location: "us-west-2",
			Status:   "Running",
		}
		assert.NotPanics(t, func() {
			display.UpdateStatus(status)
		})
	}

	// Check if the status was updated correctly
	display.StatusesMu.RLock()
	defer display.StatusesMu.RUnlock()
	assert.Len(t, display.Statuses, 1)
	assert.Equal(t, "test1", display.Statuses["test1"].ID)
}
