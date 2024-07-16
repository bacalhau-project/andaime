package display

import (
	"os"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

var statuses = []*Status{
	{
		ID:             "1",
		Type:           "EC2",
		Region:         "us-west-2",
		Zone:           "us-west-2a",
		Status:         "Running",
		DetailedStatus: "Healthy",
		ElapsedTime:    5 * time.Second,
		InstanceID:     "i-1234567890abcdef0",
		PublicIP:       "203.0.113.1",
		PrivateIP:      "10.0.0.1",
	},
	{
		ID:             "2",
		Type:           "EC2",
		Region:         "us-east-1",
		Zone:           "us-east-1b",
		Status:         "Starting",
		DetailedStatus: "Initializing",
		ElapsedTime:    2 * time.Second,
		InstanceID:     "i-0987654321fedcba0",
		PublicIP:       "203.0.113.2",
		PrivateIP:      "10.0.0.2",
	},
}

func TestDisplay(t *testing.T) {
	d := NewDisplay(0)

	// Create a channel to receive os.Signal
	sigChan := make(chan os.Signal, 1)

	// Start the display in a goroutine
	go func() {
		d.Start(sigChan)
	}()

	// Give some time for the display to start
	time.Sleep(100 * time.Millisecond)

	for _, status := range statuses {
		d.UpdateStatus(status)
	}

	// Sleep to allow for any asynchronous operations
	time.Sleep(100 * time.Millisecond)

	// Verify that the statuses were added
	d.statusesMu.RLock()
	statusCount := len(d.statuses)
	d.statusesMu.RUnlock()

	if statusCount != 2 {
		t.Errorf("Expected 2 statuses, got %d", statusCount)
	}

	// Test stopping the display
	d.Stop()

	// Wait for the display to stop
	select {
	case <-d.quit:
		// Display stopped successfully
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for display to stop")
	}
}

func TestHighlightFading(t *testing.T) {
	d := NewDisplay(0)

	// Test initial color (dark green)
	initialColor := d.getHighlightColor(d.fadeSteps)
	expectedInitialColor := tcell.NewRGBColor(0, 100, 0)
	if initialColor != expectedInitialColor {
		t.Errorf("Initial color incorrect. Expected %v, got %v", expectedInitialColor, initialColor)
	}

	// Test final color (white)
	finalColor := d.getHighlightColor(0)
	expectedFinalColor := tcell.ColorDefault
	if finalColor != expectedFinalColor {
		t.Errorf("Final color incorrect. Expected %v, got %v", expectedFinalColor, finalColor)
	}

	// Test middle color
	middleColor := d.getHighlightColor(d.fadeSteps / 2)
	r, g, b := middleColor.RGB()
	if r <= 0 || r >= 255 || g <= 100 || g >= 255 || b <= 0 || b >= 255 {
		t.Errorf("Middle color out of expected range: %v", middleColor)
	}
}

func TestUpdateStatus(t *testing.T) {
	d := NewDisplay(0)

	status := &Status{
		ID:             "1",
		Type:           "EC2",
		Region:         "us-west-2",
		Zone:           "us-west-2a",
		Status:         "Running",
		DetailedStatus: "Healthy",
		ElapsedTime:    5 * time.Second,
		InstanceID:     "i-1234567890abcdef0",
		PublicIP:       "203.0.113.1",
		PrivateIP:      "10.0.0.1",
	}

	d.UpdateStatus(status)

	d.statusesMu.RLock()
	updatedStatus, exists := d.statuses[status.ID]
	d.statusesMu.RUnlock()

	if !exists {
		t.Errorf("Status was not added to the map")
	}

	if updatedStatus.HighlightCycles != d.fadeSteps {
		t.Errorf("HighlightCycles not set correctly. Expected %d, got %d", d.fadeSteps, updatedStatus.HighlightCycles)
	}

	// Update the same status again
	d.UpdateStatus(status)

	d.statusesMu.RLock()
	updatedStatus, _ = d.statuses[status.ID]
	d.statusesMu.RUnlock()

	if updatedStatus.HighlightCycles != d.fadeSteps {
		t.Errorf("HighlightCycles not reset on update. Expected %d, got %d", d.fadeSteps, updatedStatus.HighlightCycles)
	}
}
