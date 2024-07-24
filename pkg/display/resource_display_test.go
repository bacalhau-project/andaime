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
	assert.NotNil(t, d)
	assert.NotNil(t, d.app)
	assert.NotNil(t, d.table)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.statuses)

	sigChan := make(chan os.Signal, 1)

	startComplete := make(chan struct{})
	go func() {
		d.Start(sigChan)
		close(startComplete)
	}()

	// Wait for the display to start or timeout
	select {
	case <-startComplete:
		t.Log("Display started successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for display to start")
	}

	// Update status to trigger table rendering
	t.Log("Updating status")
	updateComplete := d.UpdateStatus(&Status{
		ID:             "test-id",
		Type:           "EC2",
		Region:         "us-west-2",
		Zone:           "zone-a",
		Status:         "Running",
		DetailedStatus: "Healthy",
		ElapsedTime:    5 * time.Second,
		InstanceID:     "i-12345",
		PublicIP:       "203.0.113.1",
		PrivateIP:      "10.0.0.1",
	})

	// Wait for the update to complete or timeout
	select {
	case <-updateComplete:
		t.Log("Update completed successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for update to complete")
	}

	// Stop the display
	d.Stop()

	// Check if the table content is in the LogBox
	logContent := d.LogBox.GetText(true)
	t.Logf("LogBox content: %s", logContent)

	expectedContent := []string{
		"ID       │ Type     │ Region        │ Zone          │ Status                       │ Elapsed  │ Instance ID    │ Public IP     │ Private IP",
		"test-id  │ EC2      │ us-west-2     │ zone-a        │ Running (Healthy)            │ 5s       │ i-12345        │ 203.0.113.1   │ 10.0.0.1",
	}

	for _, expected := range expectedContent {
		assert.Contains(t, logContent, expected, "LogBox content should contain the expected table row")
	}
}
