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
	assert.NotNil(t, d.App)
	assert.NotNil(t, d.Table)
	assert.Equal(t, 10, d.TotalTasks)
	assert.False(t, d.TestMode)
}

func TestNewDisplayInternal(t *testing.T) {
	d := newDisplayInternal(5, true)
	assert.NotNil(t, d)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.App)
	assert.NotNil(t, d.Table)
	assert.Equal(t, 5, d.TotalTasks)
	assert.True(t, d.TestMode)
}

func TestDisplayStart(t *testing.T) {
	d := newDisplayInternal(1, true)
	assert.NotNil(t, d)
	assert.NotNil(t, d.App)
	assert.NotNil(t, d.Table)
	assert.NotNil(t, d.LogBox)
	assert.NotNil(t, d.Statuses)
	assert.NotNil(t, d.VirtualConsole)

	sigChan := make(chan os.Signal, 1)

	startComplete := make(chan struct{})
	go func() {
		d.Start(sigChan)
		close(startComplete)
	}()

	// Update status to trigger table rendering
	t.Log("Updating status")
	d.UpdateStatus(&Status{
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

	// Give some time for the update to complete
	time.Sleep(100 * time.Millisecond)

	// Stop the display
	d.Stop()

	// Wait for the display to stop or timeout
	select {
	case <-startComplete:
		t.Log("Display stopped successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for display to stop")
	}

	// Check if the table content is in the virtual console
	consoleContent := d.VirtualConsole.String()
	t.Logf("Virtual console content: %s", consoleContent)

	expectedContent := []string{
		"ID       │ Type     │ Region        │ Zone          │ Status                       │ Elapsed  │ Instance ID    │ Public IP     │ Private IP",
		"test-id  │ EC2      │ us-west-2     │ zone-a        │ Running (Healthy)            │ 5s       │ i-12345        │ 203.0.113.1   │ 10.0.0.1",
	}

	for _, expected := range expectedContent {
		assert.Contains(t, consoleContent, expected, "Virtual console content should contain the expected table row")
	}
}
