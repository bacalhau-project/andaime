package display

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTestDisplayStart(t *testing.T) {
	display := NewTestDisplay(1)
	sigChan := make(chan os.Signal, 1)

	// Start the display in a goroutine
	go display.Start(sigChan)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the display
	display.Stop()

	// Wait for it to stop
	display.WaitForStop()

	// Assert that no panic occurred
	assert.NotNil(t, display.Logger)
}
