package display

import (
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/logger"
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

func TestDisplayDefault(t *testing.T) {
	debugLog := logger.Get()
	debugLog.Debug("TestDisplay started")

	d := NewTestDisplay(0)

	sigChan := make(chan os.Signal, 1)
	d.Start(sigChan)

	for _, status := range statuses {
		debugLog.Debugf("Updating status: %s", status.ID)
		d.UpdateStatus(status)
	}

	d.statusesMu.RLock()
	statusCount := len(d.statuses)
	d.statusesMu.RUnlock()

	if statusCount != 2 {
		t.Errorf("Expected 2 statuses, got %d", statusCount)
	}

	d.Stop()

	select {
	case <-d.quit:
		debugLog.Debug("Display stopped successfully")
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for display to stop")
	}

	debugLog.Debug("TestDisplay finished")
}

func TestUpdateStatus(t *testing.T) {
	debugLog := logger.Get()
	debugLog.Debug("TestUpdateStatus started")

	d := NewTestDisplay(0)

	for _, status := range statuses {
		debugLog.Debugf("Updating status: %s", status.ID)
		d.UpdateStatus(status)
	}

	d.statusesMu.RLock()
	for _, status := range statuses {
		updatedStatus, exists := d.statuses[status.ID]
		if !exists {
			t.Errorf("Status with ID %s was not added to the map", status.ID)
		}
		if updatedStatus.HighlightCycles != d.fadeSteps {
			t.Errorf("HighlightCycles not set correctly for ID %s. Expected %d, got %d", status.ID, d.fadeSteps, updatedStatus.HighlightCycles)
		}
	}
	d.statusesMu.RUnlock()

	debugLog.Debug("TestUpdateStatus finished")
}

func TestHighlightFading(t *testing.T) {
	debugLog := logger.Get()
	debugLog.Debug("TestHighlightFading started")

	d := NewTestDisplay(0)

	// Test initial color (dark green)
	initialColor := d.GetHighlightColor(d.fadeSteps)
	expectedInitialColor := HighlightColor
	if initialColor != expectedInitialColor {
		t.Errorf("Initial color incorrect. Expected %v, got %v", expectedInitialColor, initialColor)
	}

	// Test final color (default)
	finalColor := d.GetHighlightColor(0)
	expectedFinalColor := tcell.ColorDefault
	if finalColor != expectedFinalColor {
		t.Errorf("Final color incorrect. Expected %v, got %v", expectedFinalColor, finalColor)
	}

	// Test middle color
	middleColor := d.GetHighlightColor(d.fadeSteps / 2)
	r, g, b := middleColor.RGB()
	if r < 0 || r > 255 || g < 100 || g > 255 || b < 0 || b > 255 {
		t.Errorf("Middle color out of expected range: %v", middleColor)
	}

	debugLog.Debug("TestHighlightFading finished")
}
