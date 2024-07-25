package display

import (
	"testing"

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

// func TestDisplayStart(t *testing.T) {
// 	d := newDisplayInternal(1, true)
// 	assert.NotNil(t, d)
// 	assert.NotNil(t, d.App)
// 	assert.NotNil(t, d.Table)
// 	assert.NotNil(t, d.LogBox)
// 	assert.NotNil(t, d.Statuses)
// 	assert.NotNil(t, d.VirtualConsole)

// 	sigChan := make(chan os.Signal, 1)

// 	startComplete := make(chan struct{})
// 	go func() {
// 		d.Start(sigChan)
// 		close(startComplete)
// 	}()

// 	// Update status to trigger table rendering
// 	t.Log("Updating status")
// 	testStatus := &Status{
// 		ID:         "test-id",
// 		Type:       "EC2",
// 		Region:     "us-west-2",
// 		Status:     "Running",
// 		InstanceID: "i-12345",
// 	}
// 	d.UpdateStatus(testStatus)

// 	// Check if the status was actually updated
// 	d.StatusesMu.RLock()
// 	updatedStatus, exists := d.Statuses[testStatus.ID]
// 	d.StatusesMu.RUnlock()
// 	t.Logf("Status exists: %v", exists)
// 	if exists {
// 		t.Logf("Updated status: %+v", updatedStatus)
// 	} else {
// 		t.Logf("All statuses: %+v", d.Statuses)
// 	}
// 	// Trigger multiple manual updates
// 	for i := 0; i < 5; i++ {
// 		d.scheduleUpdate()
// 		time.Sleep(250 * time.Millisecond)
// 	}

// 	// Stop the display
// 	d.Stop()

// 	// Wait for the display to stop or timeout
// 	select {
// 	case <-startComplete:
// 		t.Log("Display stopped successfully")
// 	case <-time.After(5 * time.Second):
// 		t.Fatal("Timeout waiting for display to stop")
// 	}

// 	// Check if key information is in the virtual console
// 	consoleContent := d.VirtualConsole.String()
// 	t.Logf("Virtual console content: %s", consoleContent)

// 	expectedContent := []string{
// 		"test-id",
// 		"EC2",
// 		"Running",
// 	}

// 	for _, expected := range expectedContent {
// 		if strings.Contains(consoleContent, expected) {
// 			t.Logf("Found expected content: %s", expected)
// 		} else {
// 			t.Logf("Missing expected content: %s", expected)
// 		}
// 		assert.Contains(t, consoleContent, expected, "Virtual console content should contain the expected information")
// 	}

// 	// Additional debugging information
// 	t.Logf("Table content: %s", d.Table.GetTitle())
// 	t.Logf("LogBox content: %s", d.LogBox.GetText(false))

// 	for _, expected := range expectedContent {
// 		if strings.Contains(consoleContent, expected) {
// 			t.Logf("Found expected content: %s", expected)
// 		} else {
// 			t.Logf("Missing expected content: %s", expected)
// 		}
// 		assert.Contains(t, consoleContent, expected, "Virtual console content should contain the expected information")
// 	}
// }
