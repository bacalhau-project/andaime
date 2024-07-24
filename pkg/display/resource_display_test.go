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
		DetailedStatus: "Healthy",
		ElapsedTime: 5 * time.Second,
		InstanceID: "i-12345",
		PublicIP: "203.0.113.1",
		PrivateIP: "10.0.0.1",
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
	assert.Contains(t, logContent, "┌──────────┬──────────┬───────────────┬───────────────┬──────────────────────────────┬──────────┬────────────────┬───────────────┬───────────────┐")
	assert.Contains(t, logContent, "│ ID       │ Type     │ Region        │ Zone          │ Status                       │ Elapsed  │ Instance ID    │ Public IP     │ Private IP    │")
	assert.Contains(t, logContent, "├──────────┼──────────┼───────────────┼───────────────┼──────────────────────────────┼──────────┼────────────────┼───────────────┼───────────────┤")
	assert.Contains(t, logContent, "│ test-id  │ EC2      │ us-west-2     │ zone-a        │ Running (Healthy)            │ 5s       │ i-12345        │ 203.0.113.1   │ 10.0.0.1      │")
	assert.Contains(t, logContent, "└──────────┴──────────┴───────────────┴───────────────┴──────────────────────────────┴──────────┴────────────────┴───────────────┴───────────────┘")
}
