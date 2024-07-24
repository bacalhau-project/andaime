package display

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestDisplayStart(t *testing.T) {
	display := NewTestDisplay(1)
	sigChan := make(chan os.Signal, 1)

	// Start the display in a goroutine
	go display.Start(sigChan)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Update status
	status := &Status{ID: "test1", Type: "test", Region: "us-west-2", Status: "Running"}
	display.UpdateStatus(status)

	// Stop the display
	display.Stop()

	// Wait for it to stop
	display.WaitForStop()

	// Assert that no panic occurred and the status was updated
	assert.NotNil(t, display.Logger)
	assert.Len(t, display.statuses, 1)
	assert.Equal(t, status.ID, display.statuses["test1"].ID)
}

func TestTestDisplayUpdateStatus(t *testing.T) {
	display := NewTestDisplay(1)
	sigChan := make(chan os.Signal, 1)

	// Start the display
	go display.Start(sigChan)
	defer display.Stop()

	// Update status multiple times
	for i := 0; i < 10; i++ {
		status := &Status{
			ID:     "test1",
			Type:   "test",
			Region: "us-west-2",
			Status: "Running",
		}
		require.NotPanics(t, func() {
			display.UpdateStatus(status)
		})
	}

	// Check if the status was updated correctly
	display.statusesMu.RLock()
	defer display.statusesMu.RUnlock()
	assert.Len(t, display.statuses, 1)
	assert.Equal(t, "test1", display.statuses["test1"].ID)
}
