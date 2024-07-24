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

	// Stop the display
	d.Stop()

	// Ensure no panic occurs
	assert.NotPanics(t, func() {
		d.WaitForStop()
	})
}
