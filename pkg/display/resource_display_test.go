package display

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDisplay(t *testing.T) {
	d := NewDisplay(10)
	assert.NotNil(t, d)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.app)
	assert.NotNil(t, d.table)
	assert.Equal(t, 10, d.totalTasks)
	assert.False(t, d.testMode)
}

func TestNewDisplayInternal(t *testing.T) {
	d := newDisplayInternal(5, true)
	assert.NotNil(t, d)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.app)
	assert.NotNil(t, d.table)
	assert.Equal(t, 5, d.totalTasks)
	assert.True(t, d.testMode)
}

func TestDisplayStart(t *testing.T) {
	d := newDisplayInternal(1, true)
	sigChan := make(chan os.Signal, 1)

	go d.Start(sigChan)

	// Give some time for the goroutines to start
	time.Sleep(100 * time.Millisecond)

	// Update status to trigger table rendering
	d.UpdateStatus(&Status{
		ID:     "test-id",
		Type:   "EC2",
		Region: "us-west-2",
		Zone:   "zone-a",
		Status: "Running",
	})

	// Give some time for the update to process
	time.Sleep(100 * time.Millisecond)

	// Stop the display
	d.Stop()

	// Ensure no panic occurs
	assert.NotPanics(t, func() {
		d.WaitForStop()
	})

	// Check if LogBox content is not empty
	assert.NotEmpty(t, d.LogBox.GetText(true))

	// Check if the table content is in the LogBox
	logContent := d.LogBox.GetText(true)
	assert.Contains(t, logContent, "test-id")
	assert.Contains(t, logContent, "EC2")
	assert.Contains(t, logContent, "us-west-2")
	assert.Contains(t, logContent, "zone-a")
	assert.Contains(t, logContent, "Running")
}
