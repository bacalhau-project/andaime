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
	updateComplete := make(chan struct{})

	t.Log("Starting display")
	go d.Start(sigChan)

	// Update status to trigger table rendering
	go func() {
		t.Log("Updating status")
		d.UpdateStatus(&Status{
			ID:              "test-id",
			Type:            "EC2",
			Region:          "us-west-2",
			Zone:            "zone-a",
			Status:          "Running",
			DetailedStatus:  "Healthy",
			ElapsedTime:     5 * time.Second,
			InstanceID:      "i-12345",
			PublicIP:        "203.0.113.1",
			PrivateIP:       "10.0.0.1",
		})
		close(updateComplete)
	}()

	// Wait for the update to complete or timeout
	t.Log("Waiting for update to complete")
	select {
	case <-updateComplete:
		t.Log("Update completed successfully")
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out waiting for update")
	}

	t.Log("Waiting for display to update")
	time.Sleep(1 * time.Second)

	// Stop the display
	d.Stop()

	// Ensure no panic occurs
	assert.NotPanics(t, func() {
		d.WaitForStop()
	})

	// Check if the table content is in the LogBox
	logContent := d.LogBox.GetText(true)
	t.Logf("LogBox content: %s", logContent) // Log the content for debugging

	expectedContent := []string{
		"ID       │ Type     │ Region        │ Zone          │ Status                       │ Elapsed  │ Instance ID    │ Public IP     │ Private IP",
		"test-id  │ EC2      │ us-west-2     │ zone-a        │ Running (Healthy)            │ 5s       │ i-12345        │ 203.0.113.1   │ 10.0.0.1",
	}

	for _, expected := range expectedContent {
		assert.Contains(t, logContent, expected, "LogBox content should contain the expected table row")
	}
}
